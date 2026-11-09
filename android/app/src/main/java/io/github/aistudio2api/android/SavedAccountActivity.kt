package io.github.aistudio2api.android
import android.app.Activity
import android.content.*
import android.os.*
import android.widget.*
import org.json.JSONObject

/** Restore route never initializes GeckoRuntime or requests browser cookies. */
class SavedAccountActivity: Activity() {
    private lateinit var status: TextView
    private var bound=false
    private val reply=Messenger(Handler(Looper.getMainLooper()) { message ->
        status.text=message.data.getString("result","NO_RESULT")
        release();true
    })
    private val connection=object: ServiceConnection {
        override fun onServiceConnected(name: ComponentName,binder: IBinder) {
            try { Messenger(binder).send(Message.obtain(null,3).apply { replyTo=reply }) }
            catch (_: Exception) {status.text="IPC_FAILURE";release()}
        }
        override fun onServiceDisconnected(name: ComponentName) { status.text="AUTH_PROCESS_DISCONNECTED";release() }
    }
    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        val layout=LinearLayout(this).apply {orientation=LinearLayout.VERTICAL;setPadding(20,70,20,40)}
        status=TextView(this).apply {text="只检查本应用加密账号；请先停止 2148 服务。不会读取浏览器 Cookie。"}
        layout.addView(Button(this).apply {text="检查已保存账号";setOnClickListener {
            if(!bound){status.text="检查中…";bound=bindService(Intent(this@SavedAccountActivity,io.github.aistudio2api.authpoc.AuthProbeService::class.java),connection,Context.BIND_AUTO_CREATE)}
        }})
        layout.addView(status);setContentView(ScrollView(this).apply {addView(layout)})
    }
    private fun release(){if(bound){unbindService(connection);bound=false}}
    override fun onDestroy(){release();super.onDestroy()}
}
