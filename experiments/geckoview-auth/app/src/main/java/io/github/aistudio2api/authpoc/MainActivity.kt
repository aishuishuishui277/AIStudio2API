package io.github.aistudio2api.authpoc

import android.app.Activity
import android.net.Uri
import android.os.Bundle
import android.view.WindowInsets
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
import org.mozilla.geckoview.*

/** Isolated browser experiment. No Go core, credential export, or stable-app access. */
class MainActivity : Activity() {
    companion object {
        private var runtime: GeckoRuntime? = null
        private const val HOME = "https://aistudio.google.com/"
    }
    private lateinit var view: GeckoView
    private lateinit var session: GeckoSession
    private lateinit var status: TextView
    private lateinit var origin: TextView
    private lateinit var back: Button
    private lateinit var forward: Button
    private var lastLoadError: String? = null
    private val sessions = mutableListOf<GeckoSession>()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        root.setOnApplyWindowInsetsListener { v, insets ->
            val bars = insets.getInsets(WindowInsets.Type.systemBars() or WindowInsets.Type.ime())
            v.setPadding(bars.left, bars.top, bars.right, bars.bottom)
            insets
        }
        origin = TextView(this).apply { textSize = 16f; setPadding(12, 8, 12, 4) }
        status = TextView(this).apply { textSize = 13f; setPadding(12, 0, 12, 4) }
        root.addView(origin)
        root.addView(status)
        val actions = LinearLayout(this)
        fun button(label: String, action: () -> Unit): Button = Button(this).also {
            it.text = label
            it.setOnClickListener { action() }
            actions.addView(it, LinearLayout.LayoutParams(0, 48.dp(), 1f))
        }
        back = button("后退") { session.goBack() }.apply { isEnabled = false }
        forward = button("前进") { session.goForward() }.apply { isEnabled = false }
        button("刷新") { session.reload() }
        button("AI Studio") { session.loadUri(HOME) }
        root.addView(actions)
        root.addView(TextView(this).apply {
            text = "独立登录 PoC · 请手动登录\n页面加载 ≠ 登录成功；auth 未验证；不导出凭据"
            textSize = 12f
            setPadding(12, 0, 12, 6)
        })
        view = GeckoView(this)
        root.addView(view, LinearLayout.LayoutParams(-1, 0, 1f))
        setContentView(root)
        root.requestApplyInsets()
        if (runtime == null) {
            runtime = GeckoRuntime.create(applicationContext, GeckoRuntimeSettings.Builder()
                .remoteDebuggingEnabled(false).consoleOutput(false).debugLogging(false)
                .loginAutofillEnabled(false).build())
        }
        session = createSession()
        session.open(runtime!!)
        view.setSession(session)
        session.loadUri(HOME)
    }

    private fun createSession(): GeckoSession = GeckoSession().also { target ->
        sessions.add(target)
        target.contentDelegate = object : GeckoSession.ContentDelegate {
            override fun onCloseRequest(s: GeckoSession) {
                if (s == session && sessions.size > 1) {
                    view.releaseSession()
                    sessions.remove(s)
                    s.close()
                    session = sessions.last()
                    view.setSession(session)
                }
            }
        }
        target.progressDelegate = object : GeckoSession.ProgressDelegate {
            override fun onPageStart(s: GeckoSession, url: String) {
                if (s == session) { lastLoadError = null; origin.text = safeOrigin(url); status.text = "正在加载…" }
            }
            override fun onPageStop(s: GeckoSession, success: Boolean) {
                if (s == session) status.text = if (success) "页面加载完成 · 登录状态待人工确认" else lastLoadError ?: "页面加载失败 · 请检查网络或页面提示"
            }
        }
        target.navigationDelegate = object : GeckoSession.NavigationDelegate {
            override fun onLocationChange(s: GeckoSession, url: String?, perms: MutableList<GeckoSession.PermissionDelegate.ContentPermission>, hasUserGesture: Boolean) {
                if (s == session) origin.text = safeOrigin(url)
            }
            override fun onCanGoBack(s: GeckoSession, enabled: Boolean) { if (s == session) back.isEnabled = enabled }
            override fun onCanGoForward(s: GeckoSession, enabled: Boolean) { if (s == session) forward.isEnabled = enabled }
            override fun onLoadRequest(s: GeckoSession, request: GeckoSession.NavigationDelegate.LoadRequest): GeckoResult<AllowOrDeny> {
                // Never launch an external app or a cleartext login page.
                return GeckoResult.fromValue(if (Uri.parse(request.uri).scheme == "https" || request.uri == "about:blank") AllowOrDeny.ALLOW else AllowOrDeny.DENY)
            }
            override fun onNewSession(s: GeckoSession, uri: String): GeckoResult<GeckoSession>? {
                if (Uri.parse(uri).scheme != "https" || sessions.size >= 4) return null
                val popup = createSession()
                view.releaseSession()
                session = popup
                view.setSession(popup)
                return GeckoResult.fromValue(popup)
            }
            override fun onLoadError(s: GeckoSession, uri: String?, error: WebRequestError): GeckoResult<String>? {
                if (s == session) {
                    lastLoadError = "加载错误 category=${error.category} code=${error.code}；未跳过 TLS 校验"
                    status.text = lastLoadError
                }
                return null
            }
        }
    }
    // Do not expose query parameters, redirect tokens, titles or page contents in diagnostics.
    private fun safeOrigin(raw: String?): String {
        val uri = raw?.let { Uri.parse(it) }
        return if (uri?.scheme == "https" && uri.host != null) "https://${uri.host}" else "页面地址隐藏"
    }
    private fun Int.dp(): Int = (this * resources.displayMetrics.density).toInt()
    override fun onDestroy() {
        view.releaseSession()
        sessions.forEach { it.close() }
        sessions.clear()
        super.onDestroy()
    }
}
