package io.github.aistudio2api.android

import android.Manifest
import android.app.*
import android.content.*
import android.content.pm.PackageManager
import android.os.*
import android.webkit.CookieManager
import android.widget.*
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URI
import java.util.concurrent.Executors

class MainActivity : Activity() {
    companion object {
        const val BASE = "http://127.0.0.1:2048"
        private const val IMPORT = 7
    }
    private lateinit var status: TextView
    private lateinit var generation: TextView
    private lateinit var importButton: Button
    private val handler = Handler(Looper.getMainLooper())
    private val worker = Executors.newSingleThreadExecutor()
    private var bound = false
    private var remote: Messenger? = null
    private var running = false
    private var startRequested = false
    private var stoppingByUser = false
    private var unexpectedDisconnect = false
    private val reply = Messenger(object : Handler(Looper.getMainLooper()) {
        override fun handleMessage(message: Message) {
            if (message.what == CoreService.QUERY) {
                val state = message.data.getString("state", "UNKNOWN")
                running = state == "RUNNING"
                if (state == "STOPPED" || state == "FAILED") startRequested = false
                status.text = "核心：$state\n${message.data.getString("detail", "")}"
                importButton.isEnabled = running
            } else if (message.what == CoreService.PROBE) {
                showMessage("Google 网络诊断（不使用账号 Cookie）：\n${message.data.getString("result", "无结果")}")
            }
        }
    })
    private val connection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName, binder: IBinder) {
            remote = Messenger(binder)
        }
        override fun onServiceDisconnected(name: ComponentName) {
            if (bound) {
                try { unbindService(this) } catch (_: IllegalArgumentException) { }
                bound = false
            }
            remote = null
            running = false
            startRequested = false
            unexpectedDisconnect = !stoppingByUser
            status.text = if (stoppingByUser) "核心：STOPPED" else "核心进程已断开，请查看日志后手动启动"
            importButton.isEnabled = false
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val content = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            val gap = (16 * resources.displayMetrics.density).toInt()
            setPadding(gap, gap, gap, gap)
        }
        val scroll = ScrollView(this).apply { addView(content) }
        setContentView(scroll)
        applySystemBars(scroll)
        content.addView(TextView(this).apply { text = "AIStudio2API"; textSize = 24f })
        status = TextView(this).apply { text = "核心：STOPPED"; textSize = 16f }
        generation = TextView(this).apply { text = "API：$BASE/v1\n端口：2048 · ARM64" }
        content.addView(status)
        content.addView(generation)
        fun button(label: String, action: () -> Unit): Button = Button(this).also {
            it.text = label
            it.setOnClickListener { action() }
            content.addView(it)
        }
        button("启动本地服务") {
            try {
                startForegroundService(Intent(this, CoreService::class.java).setAction(CoreService.START))
                startRequested = true
                stoppingByUser = false
                unexpectedDisconnect = false
                handler.postDelayed({ bindCore() }, 400)
            } catch (error: RuntimeException) { showMessage("启动失败：${error.javaClass.simpleName}") }
        }
        button("停止本地服务") { stopCore() }
        button("启动生成服务") {
            backgroundRequest("/api/control/start", "{}", "生成服务已启动")
        }
        button("打开管理页面 / 配置") { openWeb("") }
        button("账号状态") { openWeb("accounts") }
        button("模型目录") { openWeb("models") }
        importButton = button("导入 Windows 账号状态 JSON") {
            if (!running) { showMessage("请先启动本地服务"); return@button }
            startActivityForResult(Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
                type = "*/*"
                addCategory(Intent.CATEGORY_OPENABLE)
            }, IMPORT)
        }
        importButton.isEnabled = false
        button("查看服务日志") { showLogs() }
        button("测试 Google 网络连接") {
            if (!running) showMessage("请先启动本地服务")
            else try {
                remote?.send(Message.obtain(null, CoreService.PROBE).apply { replyTo = reply })
                Toast.makeText(this, "正在测试全局出口，最多等待 20 秒", Toast.LENGTH_LONG).show()
            } catch (_: RemoteException) { showMessage("核心进程已断开") }
        }
        button("复制 API 地址") {
            (getSystemService(CLIPBOARD_SERVICE) as ClipboardManager)
                .setPrimaryClip(ClipData.newPlainText("OpenAI API", "$BASE/v1"))
            showMessage("已复制 $BASE/v1")
        }
        button("停止并退出") { stopCore(); finish() }
        content.addView(TextView(this).apply {
            text = "先启动本地服务，导入 storage-state.json，再启动生成服务。模型列表以账号实际返回为准。手机网络需能连接 Google。"
        })
        if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 1)
        }
    }

    private fun bindCore() {
        if (!bound) {
            bound = bindService(Intent(this, CoreService::class.java), connection, 0)
            if (!bound && !startRequested) {
                running = false
                importButton.isEnabled = false
                if (!unexpectedDisconnect) status.text = "核心：STOPPED"
            }
        }
    }

    private val poll = object : Runnable {
        override fun run() {
            bindCore()
            try { remote?.send(Message.obtain(null, CoreService.QUERY).apply { replyTo = reply }) }
            catch (_: RemoteException) { remote = null }
            handler.postDelayed(this, 1500)
        }
    }

    override fun onResume() { super.onResume(); handler.post(poll) }
    override fun onPause() {
        handler.removeCallbacks(poll)
        startRequested = false
        if (bound) { unbindService(connection); bound = false; remote = null }
        super.onPause()
    }
    override fun onDestroy() { handler.removeCallbacksAndMessages(null); worker.shutdown(); super.onDestroy() }

    private fun stopCore() {
        if (!bound && !running && !startRequested) return
        stoppingByUser = true
        startRequested = false
        status.text = "核心：STOPPING"
        startService(Intent(this, CoreService::class.java).setAction(CoreService.STOP))
    }

    private fun openWeb(tab: String) {
        startActivity(Intent(this, WebActivity::class.java).putExtra("tab", tab))
    }

    private fun showMessage(message: String) {
        if (!isFinishing && !isDestroyed) AlertDialog.Builder(this).setMessage(message).setPositiveButton("确定", null).show()
    }

    private fun backgroundRequest(path: String, body: String, success: String) {
        if (!running) { showMessage("请先启动本地服务"); return }
        val cookies = CookieManager.getInstance().getCookie(BASE)
        worker.execute {
            val message = try { request(path, body, cookies); success }
                catch (error: Exception) { error.message ?: "请求失败" }
            runOnUiThread { showMessage(message) }
        }
    }

    private fun request(path: String, body: String, cookies: String?): String {
        val connection = URI.create(BASE + path).toURL().openConnection() as HttpURLConnection
        try {
            connection.requestMethod = "POST"
            connection.connectTimeout = 10000
            connection.readTimeout = 120000
            connection.instanceFollowRedirects = false
            connection.doOutput = true
            connection.setRequestProperty("Content-Type", "application/json")
            if (!cookies.isNullOrEmpty()) connection.setRequestProperty("Cookie", cookies)
            connection.outputStream.use { it.write(body.toByteArray(Charsets.UTF_8)) }
            val code = connection.responseCode
            val stream = if (code in 200..299) connection.inputStream else connection.errorStream
            val result = stream?.bufferedReader()?.use { it.readText() }.orEmpty()
            if (code !in 200..299) {
                if (code == 401 || code == 403) error("HTTP $code：请先在管理页登录，再重试")
                val message = try { JSONObject(result).optJSONObject("error")?.optString("message") } catch (_: Exception) { null }
                error("HTTP $code：${message?.take(500) ?: "操作失败，请查看管理页日志"}")
            }
            return result
        } finally { connection.disconnect() }
    }

    @Deprecated("Platform activity result callback")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode != IMPORT || resultCode != RESULT_OK) return
        val uri = data?.data ?: return
        val cookies = CookieManager.getInstance().getCookie(BASE)
        importButton.isEnabled = false
        worker.execute {
            val message = try {
                val bytes = contentResolver.openInputStream(uri)?.use { it.readNBytes(2 * 1024 * 1024 + 1) }
                    ?: error("无法打开所选文件")
                require(bytes.size <= 2 * 1024 * 1024) { "账号文件超过 2MB" }
                val payload = JSONObject().put("cookies", bytes.toString(Charsets.UTF_8))
                bytes.fill(0)
                request("/api/accounts/import/cookies", payload.toString(), cookies)
                "账号状态已导入。请查看账号状态与真实模型目录，再启动生成服务；导入不代表认证已通过。"
            } catch (error: Exception) { error.message ?: "导入失败" }
            runOnUiThread { importButton.isEnabled = running; showMessage(message) }
        }
    }

    private fun showLogs() {
        worker.execute {
            val exits = try {
                getSystemService(ActivityManager::class.java).getHistoricalProcessExitReasons(packageName, 0, 3)
                    .filter { it.processName.endsWith(":core") }
                    .joinToString("\n") { "核心进程退出：reason=${it.reason}, status=${it.status}, time=${it.timestamp}, PSS=${it.pss}KB" }
            } catch (_: RuntimeException) { "" }
            val file = filesDir.resolve("core/runtime/logs/aistudio2api.log")
            val error = filesDir.resolve("service-error.txt").let { if (it.isFile) it.readText() else "" }
            val text = if (file.isFile) java.io.RandomAccessFile(file, "r").use {
                it.seek(maxOf(0L, it.length() - 65536))
                val bytes = ByteArray((it.length() - it.filePointer).toInt())
                it.readFully(bytes)
                bytes.toString(Charsets.UTF_8)
            } else "尚无核心日志"
            runOnUiThread { showMessage((exits + "\n" + error + "\n" + text).trim()) }
        }
    }
}
