package io.github.aistudio2api.authpoc

import android.app.Service
import android.content.Intent
import android.os.*
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import org.json.JSONObject

object AuthNative {
    init { System.loadLibrary("authprobe") }
    @JvmStatic external fun restore(directory: String, key: String, generate: Boolean): String
    @JvmStatic external fun verify(directory: String, key: String, payload: ByteArray): String
    @JvmStatic external fun prepareUpdate(directory: String, key: String, payload: ByteArray): String
    @JvmStatic external fun commitUpdate(): String
    @JvmStatic external fun cancelUpdate(): String
}

/** One native operation at a time; a pending candidate reserves the process until commit/cancel. */
class AuthProbeService : Service() {
    private val worker = Executors.newSingleThreadExecutor()
    private val main = Handler(Looper.getMainLooper())
    private var busy = false
    private var pending = false
    private var committing = false
    private var update = false
    private var cookieReads = 0
    private var owner: Messenger? = null
    private var death: IBinder.DeathRecipient? = null
    private val cancelled = AtomicBoolean(false)
    private val timeout = Runnable { cancelPending() }
    private fun failure(code: String) = JSONObject().put("success", false).put("error", code)
        .put("category", "UNKNOWN_ERROR").put("needs_reauthentication", false)
    private fun cancelPending() {
        if (!update || committing) return
        cancelled.set(true)
        val result = try { JSONObject(AuthNative.cancelUpdate()) } catch (_: Throwable) { failure("CANCELLED") }
        if (pending) { pending = false; deliver(result, "pg-auth-3b2-result.json"); detach() }
    }
    private fun detach() {
        main.removeCallbacks(timeout)
        try { death?.let { owner?.binder?.unlinkToDeath(it, 0) } } catch (_: Exception) {}
        death = null; owner = null
    }
    private fun deliver(output: JSONObject, filename: String) {
        try {
            output.put("probe_pid", Process.myPid())
            output.put("geckoview_cookie_reads", cookieReads)
            if (!output.has("category") && !output.optBoolean("success")) output.put("category", "UNKNOWN_ERROR")
            if (!output.has("needs_reauthentication")) output.put("needs_reauthentication", output.optString("category")=="AUTH_REQUIRED")
            val text = output.toString()
            val target = android.util.AtomicFile(filesDir.resolve(filename))
            val stream = target.startWrite()
            try { stream.write(text.toByteArray()); target.finishWrite(stream) }
            catch (e: Exception) { target.failWrite(stream); throw e }
            owner?.send(Message.obtain(null, 2).apply { data = Bundle().apply { putString("result", text) } })
        } catch (_: Exception) { /* No raw exceptions, Cookie data, or server body in logs. */ }
    }
    private val messenger = Messenger(Handler(Looper.getMainLooper()) { message ->
        val recipient = message.replyTo ?: return@Handler true
        if (message.what == 7) {
            if (recipient.binder == owner?.binder) cancelPending()
            return@Handler true
        }
        if (message.what == 6) {
            if (!pending || busy || recipient.binder != owner?.binder || !recipient.binder.isBinderAlive) return@Handler true
            pending = false; busy = true; committing = true; main.removeCallbacks(timeout)
            worker.execute {
                val result = try { JSONObject(AuthNative.commitUpdate()) } catch (_: Throwable) { failure("COMMIT_RESULT_UNKNOWN").put("commit_status", "unknown") }
                main.post { busy = false; committing = false; deliver(result, "pg-auth-3b2-result.json"); detach() }
            }
            return@Handler true
        }
        if (message.what !in setOf(1,3,4,5)) return@Handler true
        if (busy || pending) {
            try { recipient.send(Message.obtain(null,2).apply { data=Bundle().apply { putString("result",failure("OPERATION_BUSY").toString()) } }) } catch (_: Exception) {}
            return@Handler true
        }
        val operation = message.what
        cookieReads=message.data.getInt("cookie_read_count",0)
        val payload = if (operation in setOf(1,5)) message.data.getByteArray("state") ?: return@Handler true else ByteArray(0)
        owner=recipient;update=operation==5;cancelled.set(false);busy=true
        death=IBinder.DeathRecipient { main.post { cancelPending() } }
        try { recipient.binder.linkToDeath(death!!,0) } catch (_: Exception) { payload.fill(0);busy=false;detach();return@Handler true }
        worker.execute {
            val output = try {
                if (cancelled.get()) failure("CANCELLED") else JSONObject(when(operation) {
                    5 -> AuthNative.prepareUpdate(filesDir.absolutePath,KeyVault.existingDataKey(this),payload)
                    1 -> AuthNative.verify(filesDir.resolve("pg-auth-3a").absolutePath,KeyVault.dataKey(this),payload)
                    else -> AuthNative.restore(filesDir.absolutePath,KeyVault.existingDataKey(this),operation==4)
                })
            } catch (_: java.security.GeneralSecurityException) { failure("KEYSTORE_UNAVAILABLE") }
            catch (_: IllegalStateException) { failure("KEYSTORE_STATE_MISSING") }
            catch (_: Throwable) { failure("NATIVE_BRIDGE_FAILURE") }
            finally { payload.fill(0) }
            main.post {
                busy=false
                if (update && cancelled.get()) {
                    try { AuthNative.cancelUpdate() } catch (_: Throwable) {}
                    deliver(failure("CANCELLED").put("committed",false).put("old_state_preserved",true),"pg-auth-3b2-result.json");detach()
                } else {
                    if (operation in setOf(3,4)) output.put("restore_route_uses_gecko",false)
                    pending=operation==5 && output.optString("stage")=="awaiting_commit_confirmation" && output.optBoolean("success")
                    deliver(output,if(operation==5) "pg-auth-3b2-result.json" else if(operation==1) "pg-auth-3a-result.json" else "pg-auth-3b1-result.json")
                    if (pending) main.postDelayed(timeout,110000) else detach()
                }
            }
        }
        true
    })
    override fun onBind(intent: Intent?): IBinder=messenger.binder
    override fun onUnbind(intent: Intent?): Boolean { cancelPending();return false }
    override fun onDestroy() { cancelPending();worker.shutdown();super.onDestroy() }
}
