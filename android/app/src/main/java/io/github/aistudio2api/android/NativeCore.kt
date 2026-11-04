package io.github.aistudio2api.android

object NativeCore {
    init { System.loadLibrary("aistudio2api") }
    @JvmStatic external fun start(directory: String, key: String): String
    @JvmStatic external fun stop(): String
    @JvmStatic external fun status(): String
    @JvmStatic external fun probe(): String
}
