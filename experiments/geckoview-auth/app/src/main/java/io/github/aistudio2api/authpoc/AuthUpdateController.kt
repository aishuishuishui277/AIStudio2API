package io.github.aistudio2api.authpoc

import android.app.Activity
import android.app.AlertDialog
import android.content.*
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.*
import android.widget.*
import org.json.JSONObject

/** No Cookie request until the user confirms completed same-account login. */
class AuthUpdateController(private val activity: Activity, private val button: Button,
    private val cancel: Button, private val status: TextView, private val inspector: CookieInspector) {
    private var waiting = false
    private var binding = false
    private var closed = false
    private var committing = false
    private var payload: ByteArray? = null
    private var remote: Messenger? = null
    private var dialog: AlertDialog? = null
    private val reply: Messenger = Messenger(Handler(Looper.getMainLooper()) { message ->
        if (message.what == 2 && !closed) {
            try {
                val result = JSONObject(message.data.getString("result") ?: "{}")
                status.text = result.toString(2)
                if (result.optString("stage") == "awaiting_commit_confirmation" && result.optBoolean("success")) {
                    dialog = AlertDialog.Builder(activity).setTitle("同一账号已校验通过")
                        .setMessage("新状态已加密暂存，账号、模型和 WAA 校验通过。现在替换固定账号的认证状态？取消会保留旧状态。确认后进入不可取消的原子提交。")
                        .setPositiveButton("确认替换") { _, _ ->
                            committing = true; cancel.isEnabled = false
                            status.text = "正在原子提交；完成前请保持应用打开。"
                            try { remote?.send(Message.obtain(null,6).apply { replyTo=reply }) }
                            catch (_: Exception) { status.text="提交结果未知；重新检查已保存账号。";release() }
                        }
                        .setNegativeButton("取消") { _, _ -> cancelUpdate() }
                        .setOnCancelListener { cancelUpdate() }.show()
                } else {
                    dialog?.dismiss();dialog=null
                    if (result.optBoolean("committed")) status.text="已提交（请返回检查已保存账号）\n"+result.toString(2)
                    release()
                }
            } catch (_: Exception) { status.text="INVALID_RESULT";cancelUpdate() }
        }
        true
    })
    private val connection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName, binder: IBinder) {
            val bytes=payload ?: return
            remote=Messenger(binder)
            try {
                remote!!.send(Message.obtain(null,5).apply {
                    replyTo=reply;data=Bundle().apply {
                        putByteArray("state",bytes)
                        putInt("cookie_read_count",activity.getSharedPreferences("cookie-audit",Activity.MODE_PRIVATE).getInt("read-attempts",0))
                    }
                })
                status.text="正在私有暂存中验证同一账号、模型和 WAA；旧状态尚未修改。"
            } catch (_: Exception) { status.text="IPC_FAILURE";cancelUpdate() }
            finally { bytes.fill(0);payload=null }
        }
        override fun onServiceDisconnected(name: ComponentName) { status.text=if(committing) "提交结果未知；重新检查已保存账号。" else "PG 进程退出；未提交状态不会在重启时发布。";release() }
        override fun onNullBinding(name: ComponentName) { status.text="SERVICE_UNAVAILABLE";release() }
        override fun onBindingDied(name: ComponentName) { onServiceDisconnected(name) }
    }
    init {
        cancel.isEnabled=false
        cancel.setOnClickListener { cancelUpdate() }
        inspector.onAuthResult = { data ->
            if (waiting && !closed) {
                waiting=false
                try {
                    if (data.optString("type")=="authState") {
                        val state=data.getJSONObject("state")
                        require(state.keys().asSequence().toSet()==setOf("cookies"))
                        val bytes=state.toString().toByteArray(Charsets.UTF_8);require(bytes.size<=256*1024)
                        payload=bytes
                        binding=activity.bindService(Intent(activity,AuthProbeService::class.java),connection,Context.BIND_AUTO_CREATE)
                        if(!binding){status.text="SERVICE_UNAVAILABLE";release()}
                    } else {
                        val code=data.optString("code")
                        status.text=if(code in setOf("PARTITION_UNSUPPORTED","COOKIE_SAMESITE_INVALID","COOKIE_EXPIRED","COOKIE_HOST_ONLY_CONFLICT","COOKIE_API_UNAVAILABLE","COOKIE_QUERY_FAILED","AUTH_MESSAGE_TIMEOUT")) code else "INVALID_EXTENSION_RESULT"
                        release()
                    }
                } catch (_: Exception) { status.text="INVALID_CANDIDATE_FORMAT";release() }
            }
        }
        button.setOnClickListener {
            if (!binding && !waiting && !closed) {
                dialog=AlertDialog.Builder(activity).setTitle("确认已完成 Google 登录")
                    .setMessage("请确认当前浏览器已登录原来的同一个 Google 账号。确认后只在应用内部读取 Cookie 并验证；验证完成后还需确认替换。")
                    .setPositiveButton("已登录，开始验证") { _, _ ->
                        val network=activity.getSystemService(ConnectivityManager::class.java)
                        if(network.getNetworkCapabilities(network.activeNetwork)?.hasTransport(NetworkCapabilities.TRANSPORT_VPN)!=true){status.text="NETWORK_FAILURE：当前应用没有 VPN；未读取 Cookie。"}
                        else {
                            waiting=true;button.isEnabled=false;cancel.isEnabled=true
                            status.text="用户已确认；正在内部读取候选状态。"
                            if(!inspector.requestAuth()){status.text="扩展未连接，请稍后手动重试。";release()}
                        }
                    }.setNegativeButton("取消") {_,_-> status.text="已取消；未读取 Cookie、未修改旧状态。"}.show()
            }
        }
    }
    private fun cancelUpdate() {
        if(committing)return
        waiting=false
        try {remote?.send(Message.obtain(null,7).apply {replyTo=reply})}catch (_:Exception){}
        dialog?.dismiss();dialog=null
        status.text="已取消；未授权原子提交，旧状态保持不变。"
        release()
    }
    private fun release() {
        payload?.fill(0);payload=null;waiting=false
        if(binding){activity.unbindService(connection);binding=false}
        remote=null;committing=false
        button.isEnabled=!closed;cancel.isEnabled=false
    }
    fun close(){closed=true;inspector.onAuthResult=null;if(!committing)cancelUpdate();dialog?.dismiss();dialog=null;release()}
}
