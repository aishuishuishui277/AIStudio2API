package io.github.aistudio2api.authpoc

import android.app.Activity
import android.app.AlertDialog
import android.os.Handler
import android.os.Looper
import android.widget.Button
import android.widget.ScrollView
import android.widget.TextView
import org.json.JSONObject
import org.mozilla.geckoview.*

/** Only whitelisted, value-free metadata is accepted. No raw message logging. */
class CookieInspector(private val activity: Activity, private val button: Button, private val status: TextView) {
    private val handler = Handler(Looper.getMainLooper())
    private var port: WebExtension.Port? = null
    private var extension: WebExtension? = null
    private var pending = 0
    private var sequence = 0
    private var closed = false
    private val extensionId = "cookie-metadata@aistudio2api.local"
    private val host = "cookie_metadata"
    private val timeout = Runnable {
        if (pending != 0) { pending = 0; status.text = "检查超时：Native Messaging 未返回结果"; button.isEnabled = port != null }
    }
    private fun trusted(sender: WebExtension.MessageSender) = sender.webExtension.id == extensionId &&
        sender.session == null && sender.environmentType == WebExtension.MessageSender.ENV_TYPE_EXTENSION

    fun install(runtime: GeckoRuntime) {
        button.isEnabled = false
        status.text = "正在安装内置扩展…"
        button.setOnClickListener {
            if (port != null && pending == 0) {
                pending = ++sequence
                button.isEnabled = false
                status.text = "正在检查 Cookie 元数据（不含值）…"
                port!!.postMessage(JSONObject().put("action", "inspect").put("id", pending))
                handler.postDelayed(timeout, 15000)
            }
        }
        // Built-in installation restarts only this extension, not the browser profile.
        runtime.webExtensionController.installBuiltIn("resource://android/assets/cookie-metadata/").accept({ ext ->
            if (closed) return@accept
            if (ext == null) { status.text = "内置扩展安装失败：空结果"; return@accept }
            extension = ext
            status.text = "扩展已安装 · 等待 Native Messaging"
            ext.setMessageDelegate(object : WebExtension.MessageDelegate {
                override fun onConnect(p: WebExtension.Port) {
                    if (closed || p.name != host || !trusted(p.sender)) { p.disconnect(); return }
                    port = p
                    p.setDelegate(object : WebExtension.PortDelegate {
                        override fun onDisconnect(disconnected: WebExtension.Port) {
                            if (port == disconnected) { port = null; button.isEnabled = false; status.text = "Native Messaging 已断开" }
                        }
                    })
                    status.text = "扩展已安装 · Native Messaging 已连接"
                    button.isEnabled = pending == 0
                }
                override fun onMessage(nativeApp: String, message: Any, sender: WebExtension.MessageSender): GeckoResult<Any>? {
                    if (closed || nativeApp != host || !trusted(sender) || pending == 0) return GeckoResult.fromValue(false as Any)
                    try {
                        val data = message as JSONObject
                        require(data.getInt("id") == pending)
                        if (data.getString("type") == "error") {
                            keys(data, setOf("type", "id", "code"))
                            require(data.getString("code") in setOf("COOKIE_QUERY_FAILED", "COOKIE_API_UNAVAILABLE"))
                            status.text = "扩展错误：${data.getString("code")}"
                        } else {
                            validate(data)
                            // Only the schema-validated metadata reaches UI or this private evidence file.
                            activity.filesDir.resolve("cookie-metadata.json").writeText(data.toString(2))
                            status.text = "读取成功：${data.getInt("count")} 个 Cookie · 结构校验通过 · 不含值"
                            val text = TextView(activity).apply { setPadding(24, 16, 24, 16); text = data.toString(2); textSize = 12f }
                            AlertDialog.Builder(activity).setTitle("Cookie 元数据：${data.getInt("count")} 个")
                                .setView(ScrollView(activity).apply { addView(text) }).setPositiveButton("关闭", null).show()
                        }
                        pending = 0
                        handler.removeCallbacks(timeout)
                        button.isEnabled = port != null
                        return GeckoResult.fromValue(true as Any)
                    } catch (_: Exception) {
                        pending = 0
                        handler.removeCallbacks(timeout)
                        button.isEnabled = port != null
                        status.text = "拒绝结果：来源、字段或结构校验失败（未记录原始数据）"
                        return GeckoResult.fromValue(false as Any)
                    }
                }
            }, host)
        }, { _ -> if (!closed) status.text = "内置扩展安装失败（未记录原始异常）" })
    }
    private fun keys(obj: JSONObject, expected: Set<String>) { require(obj.keys().asSequence().toSet() == expected) }
    private fun validate(data: JSONObject) {
        keys(data, setOf("type", "id", "count", "cookies"))
        require(data.getString("type") == "metadata")
        val cookies = data.getJSONArray("cookies")
        require(cookies.length() == data.getInt("count") && cookies.length() <= 1000)
        val booleans = setOf("secure", "httpOnly", "session", "hostOnly", "hasExpiration", "hasPartitionKey", "hasFirstPartyDomain")
        for (index in 0 until cookies.length()) {
            val c = cookies.getJSONObject(index)
            keys(c, booleans + setOf("name", "domain", "path", "sameSite"))
            booleans.forEach { require(c.get(it) is Boolean) }
            require(c.get("name") is String && c.getString("name").matches(Regex("[!#$%&'*+.^_`|~0-9A-Za-z-]{1,256}")))
            require(c.get("domain") is String && c.getString("domain").removePrefix(".") in setOf("google.com", "accounts.google.com", "aistudio.google.com"))
            require(c.get("path") is String && c.getString("path").startsWith("/") && c.getString("path").length <= 2048)
            require(c.get("sameSite") is String && c.getString("sameSite") in setOf("no_restriction", "lax", "strict", "unspecified"))
        }
    }
    fun close() {
        closed = true
        handler.removeCallbacksAndMessages(null)
        extension?.setMessageDelegate(null, host)
        port?.disconnect()
        port = null
    }
}
