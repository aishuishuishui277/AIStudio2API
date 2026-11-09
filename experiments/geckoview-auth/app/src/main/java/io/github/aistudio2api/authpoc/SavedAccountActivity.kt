package io.github.aistudio2api.authpoc

import android.app.Activity
import android.content.*
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.*
import android.view.WindowInsets
import android.widget.*
import org.json.JSONObject

/** Native-only entry: constructing this Activity cannot construct Gecko or send extension messages. */
class SavedAccountActivity : Activity() {
    private lateinit var status: TextView
    private lateinit var start: Button
    private lateinit var generate: Button
    private lateinit var browser: Button
    private lateinit var update: Button
    private var binding = false
    private var operation = 3
    private var automatic = false
    private val preferences by lazy { getSharedPreferences("saved-account-3b2", MODE_PRIVATE) }
    private val reply = Messenger(Handler(Looper.getMainLooper()) { message ->
        if (message.what == 2) {
            try {
                val result = JSONObject(message.data.getString("result") ?: "{}")
                result.put("automatic_restore", automatic)
                val success = result.optBoolean("success")
                if (operation == 3 && success) preferences.edit().putBoolean("restore-on-launch", true).commit()
                else if (operation == 3 && result.optBoolean("needs_reauthentication")) preferences.edit().putBoolean("restore-on-launch", false).commit()
                status.text = (if (success) "Ready 检查通过；本次探针已关闭，不是常驻服务。\n" else "恢复失败；未回退到浏览器登录。\n") + result.toString(2)
            } catch (_: Exception) { status.text = "INVALID_RESULT（未记录原文）" }
            release()
        }
        true
    })
    private val connection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName, binder: IBinder) {
            try { Messenger(binder).send(Message.obtain(null, operation).apply { replyTo = reply; data=Bundle().apply { putInt("cookie_read_count",getSharedPreferences("cookie-audit",MODE_PRIVATE).getInt("read-attempts",0)) } }) }
            catch (_: Exception) { status.text = "IPC_FAILURE"; release() }
        }
        override fun onServiceDisconnected(name: ComponentName) { status.text = "PROCESS_INTERRUPTED；未自动重试"; release() }
        override fun onNullBinding(name: ComponentName) { status.text = "SERVICE_UNAVAILABLE"; release() }
        override fun onBindingDied(name: ComponentName) { status.text = "PROCESS_INTERRUPTED；未自动重试"; release() }
    }
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(16, 0, 16, 0) }
        root.setOnApplyWindowInsetsListener { view, insets ->
            val bars = insets.getInsets(WindowInsets.Type.systemBars())
            view.setPadding(16 + bars.left, bars.top, 16 + bars.right, bars.bottom); insets
        }
        root.addView(TextView(this).apply { text = "3B-2 认证检查与手动更新\n不启动 GeckoView，不读取浏览器 Cookie。首次恢复成功后，冷启动自动检查 Ready；生成仍须单独点击。"; textSize = 15f })
        start = Button(this).apply { text = "检查已保存账号"; setOnClickListener { begin(false, false) } }
        generate = Button(this).apply { text = "验证已保存账号生成（一次）"; setOnClickListener { begin(true, false) } }
        browser = Button(this).apply { text = "重新登录 Google"; setOnClickListener { startActivity(Intent(this@SavedAccountActivity, MainActivity::class.java)) } }
        update = Button(this).apply { text = "手动更新 PG 认证"; setOnClickListener { startActivity(Intent(this@SavedAccountActivity, MainActivity::class.java)) } }
        root.addView(start); root.addView(generate); root.addView(browser); root.addView(update)
        status = TextView(this).apply { text = "待启动；本页没有 Cookie 获取或导入入口。"; textSize = 12f; setTextIsSelectable(true) }
        root.addView(ScrollView(this).apply { addView(status) }, LinearLayout.LayoutParams(-1, 0, 1f))
        setContentView(root)
        if (savedInstanceState == null && preferences.getBoolean("restore-on-launch", false)) begin(false, true)
    }
    private fun begin(withGeneration: Boolean, auto: Boolean) {
        if (binding) return
        val network = getSystemService(ConnectivityManager::class.java)
        if (network.getNetworkCapabilities(network.activeNetwork)?.hasTransport(NetworkCapabilities.TRANSPORT_VPN) != true) {
            status.text = "VPN_NOT_ACTIVE：停止网络测试，请检查 FlClash 应用分流。"; return
        }
        automatic = auto
        operation = if (withGeneration) 4 else 3
        start.isEnabled = false; generate.isEnabled = false; browser.isEnabled = false; update.isEnabled = false
        status.text = "正在从独立持久目录恢复，最多 180 秒；不读取 Gecko Cookie。"
        binding = bindService(Intent(this, AuthProbeService::class.java), connection, BIND_AUTO_CREATE)
        if (!binding) { status.text = "SERVICE_UNAVAILABLE"; release() }
    }
    private fun release() {
        if (binding) { unbindService(connection); binding = false }
        start.isEnabled = true; generate.isEnabled = true; browser.isEnabled = true; update.isEnabled = true
    }
    override fun onDestroy() { release(); super.onDestroy() }
}
