package io.github.aistudio2api.android

import android.app.*
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.*
import java.util.concurrent.Executors

class CoreService : Service() {
    companion object {
        const val START = "io.github.aistudio2api.START"
        const val STOP = "io.github.aistudio2api.STOP"
        const val QUERY = 1
        const val PROBE = 2
        private const val CHANNEL = "local-api"
        private const val NOTIFICATION = 2048
    }

    private val handler = Handler(Looper.getMainLooper())
    private val worker = Executors.newSingleThreadExecutor()
    private var state = "STOPPED"
    private var detail = ""
    private var stopping = false
    private var foreground = false
    private var probing = false
    private val messenger = Messenger(object : Handler(Looper.getMainLooper()) {
        override fun handleMessage(message: Message) {
            if (message.what == QUERY) {
                val reply = Message.obtain(null, QUERY).apply {
                    data = Bundle().apply { putString("state", state); putString("detail", detail) }
                }
                try { message.replyTo?.send(reply) } catch (_: RemoteException) { }
            } else if (message.what == PROBE && state == "RUNNING" && !stopping && !probing) {
                probing = true
                val destination = message.replyTo
                Thread {
                    val result = try { NativeCore.probe() } catch (error: Throwable) { error.javaClass.simpleName }
                    val reply = Message.obtain(null, PROBE).apply {
                        data = Bundle().apply { putString("result", result) }
                    }
                    try { destination?.send(reply) } catch (_: RemoteException) { }
                    handler.post { probing = false }
                }.start()
            } else super.handleMessage(message)
        }
    })

    override fun onCreate() {
        super.onCreate()
        getSystemService(NotificationManager::class.java).createNotificationChannel(
            NotificationChannel(CHANNEL, "本地 API 服务", NotificationManager.IMPORTANCE_LOW))
    }

    override fun onBind(intent: Intent): IBinder = messenger.binder

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == STOP) {
            stopCore()
            return START_NOT_STICKY
        }
        if (intent?.action != START || state == "RUNNING" || state == "STARTING" || stopping) return START_NOT_STICKY
        state = "STARTING"
        detail = "正在启动 127.0.0.1:2048"
        try {
            startForeground(NOTIFICATION, notification(detail), ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
            foreground = true
        } catch (error: RuntimeException) {
            fail("前台服务启动失败：${error.javaClass.simpleName}")
            return START_NOT_STICKY
        }
        worker.execute {
            val error = try {
                val stopped = NativeCore.stop()
                if (stopped.isNotEmpty()) stopped
                else NativeCore.start(filesDir.resolve("core").absolutePath, KeyVault.dataKey(this))
            }
                catch (error: Throwable) { "本地核心加载失败：${error.javaClass.simpleName}: ${error.message}" }
            handler.post {
                if (stopping) return@post
                if (error.isNotEmpty()) fail(error)
                else {
                    state = "RUNNING"
                    detail = "127.0.0.1:2048 · 请在管理页启动生成服务"
                    getSystemService(NotificationManager::class.java).notify(NOTIFICATION, notification(detail))
                    handler.postDelayed(monitor, 2000)
                }
            }
        }
        return START_NOT_STICKY
    }

    private val monitor = object : Runnable {
        override fun run() {
            if (state != "RUNNING" || stopping) return
            val status = try { NativeCore.status() } catch (_: Throwable) { "FAILED: native core unavailable" }
            if (status.startsWith("FAILED") || status == "STOPPED") fail(status)
            else handler.postDelayed(this, 2000)
        }
    }

    private fun notification(text: String): Notification {
        val open = PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP), PendingIntent.FLAG_IMMUTABLE)
        val stop = PendingIntent.getService(this, 1, Intent(this, CoreService::class.java).setAction(STOP), PendingIntent.FLAG_IMMUTABLE)
        return Notification.Builder(this, CHANNEL).setSmallIcon(android.R.drawable.stat_notify_sync)
            .setContentTitle("AIStudio2API 本地服务").setContentText(text).setContentIntent(open)
            .setOngoing(true).setCategory(Notification.CATEGORY_SERVICE)
            .addAction(Notification.Action.Builder(null, "停止", stop).build()).build()
    }

    private fun fail(message: String) {
        state = "FAILED"
        detail = message
        filesDir.resolve("service-error.txt").writeText(message)
        if (foreground) stopForeground(STOP_FOREGROUND_REMOVE)
        foreground = false
        stopSelf()
    }

    private fun stopCore() {
        if (stopping) return
        stopping = true
        state = "STOPPING"
        detail = "正在关闭连接和数据库"
        handler.removeCallbacks(monitor)
        worker.execute {
            val error = try { NativeCore.stop() } catch (error: Throwable) { error.javaClass.simpleName }
            handler.post {
                state = if (error.isEmpty()) "STOPPED" else "FAILED"
                detail = error
                if (error.isNotEmpty()) filesDir.resolve("service-error.txt").writeText(error)
                if (foreground) stopForeground(STOP_FOREGROUND_REMOVE)
                foreground = false
                stopping = false
                stopSelf()
            }
        }
    }

    override fun onTimeout(startId: Int, fgsType: Int) { stopCore() }

    override fun onDestroy() {
        handler.removeCallbacks(monitor)
        if (!stopping && state != "STOPPED") worker.execute { try { NativeCore.stop() } catch (_: Throwable) { } }
        worker.shutdown()
        super.onDestroy()
    }
}
