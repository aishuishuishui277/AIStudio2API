package io.github.aistudio2api.android

import android.app.Activity
import android.view.View
import android.view.WindowInsets
import android.view.WindowInsetsController

fun Activity.applySystemBars(content: View) {
    val appearance = WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS or WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS
    window.insetsController?.setSystemBarsAppearance(appearance, appearance)
    content.setOnApplyWindowInsetsListener { view, insets ->
        val bars = insets.getInsets(WindowInsets.Type.systemBars())
        view.setPadding(bars.left, bars.top, bars.right, bars.bottom)
        insets
    }
    content.requestApplyInsets()
}
