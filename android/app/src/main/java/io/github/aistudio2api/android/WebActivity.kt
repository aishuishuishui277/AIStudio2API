package io.github.aistudio2api.android

import android.app.Activity
import android.os.Bundle
import android.webkit.*
import android.widget.FrameLayout

class WebActivity : Activity() {
    private lateinit var web: WebView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        web = WebView(this)
        val frame = FrameLayout(this).apply { addView(web) }
        setContentView(frame)
        applySystemBars(frame)
        WebView.setWebContentsDebuggingEnabled(false)
        web.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            allowFileAccess = false
            allowContentAccess = false
            mixedContentMode = android.webkit.WebSettings.MIXED_CONTENT_NEVER_ALLOW
            setSupportMultipleWindows(false)
        }
        web.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                val uri = request.url
                return uri.scheme != "http" || uri.host != "127.0.0.1" || uri.port !in listOf(2048, 2049)
            }
        }
        val tab = intent.getStringExtra("tab").orEmpty().takeIf { it in listOf("accounts", "models", "logs", "settings") }.orEmpty()
        web.loadUrl(MainActivity.BASE + if (tab.isEmpty()) "/" else "/?tab=$tab")
    }

    @Deprecated("Platform back callback")
    override fun onBackPressed() { if (web.canGoBack()) web.goBack() else super.onBackPressed() }

    override fun onDestroy() { web.stopLoading(); web.destroy(); super.onDestroy() }
}
