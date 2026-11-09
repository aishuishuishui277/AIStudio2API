package io.github.aistudio2api.authpoc

import android.app.Service
import android.content.Intent
import android.os.*
import java.util.concurrent.Executors
import org.json.JSONObject

object AuthNative {
    init { System.loadLibrary("authprobe") }
    @JvmStatic external fun restore(directory: String, key: String, generate: Boolean): String
    @JvmStatic external fun verify(directory: String, key: String, payload: ByteArray): String
}

/** Bound, non-exported, separate process. No browser profile or stable app access. */
class AuthProbeService : Service() {
    private val worker = Executors.newSingleThreadExecutor()
    private var busy = false
    private val main = Handler(Looper.getMainLooper())
    private val messenger = Messenger(Handler(Looper.getMainLooper()) { message ->
        if (message.what !in setOf(1, 3, 4) || busy) return@Handler true
        val recipient = message.replyTo ?: return@Handler true
        val restore = message.what != 1
        val generate = message.what == 4
        val payload = if (restore) ByteArray(0) else message.data.getByteArray("state") ?: return@Handler true
        busy = true
        worker.execute {
            val result = try {
                if (restore) AuthNative.restore(filesDir.absolutePath, KeyVault.existingDataKey(this), generate)
                else AuthNative.verify(filesDir.resolve("pg-auth-3a").absolutePath, KeyVault.dataKey(this), payload)
            } catch (_: java.security.GeneralSecurityException) { "{\"success\":false,\"error\":\"KEYSTORE_UNAVAILABLE\"}" }
            catch (_: IllegalStateException) { "{\"success\":false,\"error\":\"KEYSTORE_STATE_MISSING\"}" }
            catch (_: Throwable) { "{\"success\":false,\"error\":\"NATIVE_BRIDGE_FAILURE\"}" }
            finally { payload.fill(0) }
            main.post {
                busy = false
                try {
                    // Native output consists solely of fixed status fields and public model IDs.
                    val output = JSONObject(result).put("probe_pid", Process.myPid())
                    if (restore) {
                        output.put("geckoview_cookie_reads", getSharedPreferences("cookie-audit", MODE_PRIVATE).getInt("read-attempts", 0))
                        output.put("restore_route_uses_gecko", false)
                    }
                    val safeResult = output.toString()
                    val target = android.util.AtomicFile(filesDir.resolve(if (restore) "pg-auth-3b1-result.json" else "pg-auth-3a-result.json"))
                    val stream = target.startWrite()
                    try { stream.write(safeResult.toByteArray()); target.finishWrite(stream) }
                    catch (e: Exception) { target.failWrite(stream); throw e }
                    recipient.send(Message.obtain(null, 2).apply { data = Bundle().apply { putString("result", safeResult) } })
                } catch (_: Exception) { /* Do not print IPC data or exceptions. */ }
            }
        }
        true
    })
    override fun onBind(intent: Intent?): IBinder = messenger.binder
    override fun onDestroy() { worker.shutdown(); super.onDestroy() }
}
