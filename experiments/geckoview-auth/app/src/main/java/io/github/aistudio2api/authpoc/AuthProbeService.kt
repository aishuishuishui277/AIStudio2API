package io.github.aistudio2api.authpoc

import android.app.Service
import android.content.Intent
import android.os.*
import java.util.concurrent.Executors
import org.json.JSONObject

object AuthNative {
    init { System.loadLibrary("authprobe") }
    @JvmStatic external fun verify(directory: String, key: String, payload: ByteArray): String
}

/** Bound, non-exported, separate process. No browser profile or stable app access. */
class AuthProbeService : Service() {
    private val worker = Executors.newSingleThreadExecutor()
    private var busy = false
    private val main = Handler(Looper.getMainLooper())
    private val messenger = Messenger(Handler(Looper.getMainLooper()) { message ->
        if (message.what != 1 || busy) return@Handler true
        val recipient = message.replyTo ?: return@Handler true
        val payload = message.data.getByteArray("state") ?: return@Handler true
        busy = true
        worker.execute {
            val result = try {
                AuthNative.verify(filesDir.resolve("pg-auth-3a").absolutePath, KeyVault.dataKey(this), payload)
            } catch (_: Throwable) { "{\"success\":false,\"error\":\"NATIVE_BRIDGE_FAILURE\"}" }
            finally { payload.fill(0) }
            main.post {
                busy = false
                try {
                    // Native output consists solely of fixed status fields and public model IDs.
                    JSONObject(result)
                    filesDir.resolve("pg-auth-3a-result.json").writeText(result)
                    recipient.send(Message.obtain(null, 2).apply { data = Bundle().apply { putString("result", result) } })
                } catch (_: Exception) { /* Do not print IPC data or exceptions. */ }
            }
        }
        true
    })
    override fun onBind(intent: Intent?): IBinder = messenger.binder
    override fun onDestroy() { worker.shutdown(); super.onDestroy() }
}
