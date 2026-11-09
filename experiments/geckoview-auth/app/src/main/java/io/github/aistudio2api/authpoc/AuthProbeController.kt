package io.github.aistudio2api.authpoc

import android.app.Activity
import android.app.AlertDialog
import android.content.*
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.*
import android.widget.*
import org.json.JSONObject

class AuthProbeController(private val activity: Activity, private val button: Button,
    private val status: TextView, private val inspector: CookieInspector) {
    private var waitingForUserRequest = false
    private var binding = false
    private var payload: ByteArray? = null
    private val reply = Messenger(Handler(Looper.getMainLooper()) { message ->
        if (message.what == 2) {
            val result = message.data.getString("result") ?: "{}"
            status.text = "3A 验证已结束（仅脱敏结果）"
            val text = TextView(activity).apply { text = result; textSize = 12f; setPadding(20, 16, 20, 16) }
            AlertDialog.Builder(activity).setTitle("PG 认证兼容性结果")
                .setView(ScrollView(activity).apply { addView(text) }).setPositiveButton("关闭", null).show()
            release()
            button.isEnabled = true
        }
        true
    })
    private val connection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName, binder: IBinder) {
            val bytes = payload ?: return
            try {
                Messenger(binder).send(Message.obtain(null, 1).apply {
                    replyTo = reply
                    data = Bundle().apply { putByteArray("state", bytes) }
                })
                status.text = "正在进行真实 PG 验证，最多 180 秒；请保持应用前台"
            } catch (_: Exception) { status.text = "实验 IPC 失败"; release(); button.isEnabled = true }
            finally { bytes.fill(0); payload = null }
        }
        override fun onServiceDisconnected(name: ComponentName) { status.text = "实验进程中断；未自动重试"; release(); button.isEnabled = true }
    }
    init {
        inspector.onAuthResult = { data ->
            if (waitingForUserRequest) {
                waitingForUserRequest = false
                try {
                    if (data.optString("type") == "authState") {
                        val state = data.getJSONObject("state")
                        require(state.keys().asSequence().toSet() == setOf("cookies"))
                        val bytes = state.toString().toByteArray(Charsets.UTF_8)
                        require(bytes.size <= 256 * 1024)
                        payload = bytes
                        binding = activity.bindService(Intent(activity, AuthProbeService::class.java), connection, Context.BIND_AUTO_CREATE)
                        if (!binding) { release(); status.text = "实验服务启动失败"; button.isEnabled = true }
                    } else {
                        val code = data.optString("code")
                        status.text = if (code in setOf("PARTITION_UNSUPPORTED", "COOKIE_SAMESITE_INVALID", "COOKIE_EXPIRED", "COOKIE_HOST_ONLY_CONFLICT", "COOKIE_API_UNAVAILABLE", "COOKIE_QUERY_FAILED", "AUTH_MESSAGE_TIMEOUT")) "验证停止：$code" else "验证停止：扩展返回无效结果"
                        button.isEnabled = true
                    }
                } catch (_: Exception) { release(); status.text = "认证传递格式无效（未记录原文）"; button.isEnabled = true }
            }
        }
        button.setOnClickListener {
            // This listener is the ONLY trigger for requesting real Cookie values.
            val manager = activity.getSystemService(ConnectivityManager::class.java)
            if (manager.getNetworkCapabilities(manager.activeNetwork)?.hasTransport(NetworkCapabilities.TRANSPORT_VPN) != true) {
                status.text = "停止：当前应用未使用 VPN。请确认 FlClash 应用分流后再点按钮。"
            } else if (!waitingForUserRequest && !binding) {
                waitingForUserRequest = true
                button.isEnabled = false
                status.text = "用户已触发：正在应用内部传递认证状态，不显示凭据"
                if (!inspector.requestAuth()) { waitingForUserRequest = false; button.isEnabled = true; status.text = "扩展尚未连接，请稍后手动重试" }
            }
        }
    }
    private fun release() {
        payload?.fill(0); payload = null; waitingForUserRequest = false
        if (binding) { activity.unbindService(connection); binding = false }
    }
    fun close() { inspector.onAuthResult = null; release() }
}
