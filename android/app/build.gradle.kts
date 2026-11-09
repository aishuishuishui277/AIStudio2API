plugins { id("com.android.application") }

android {
    namespace = "io.github.aistudio2api.android"
    compileSdk { version = release(37) { minorApiLevel = 2 } }
    defaultConfig {
        applicationId = "io.github.aistudio2api.integration3c1"
        minSdk = 35
        targetSdk = 35
        versionCode = 1
        versionName = "0.6.0-integration-3c1"
        ndk { abiFilters += "arm64-v8a" }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    packaging { jniLibs { useLegacyPackaging = false } }
    buildTypes {
        release { isMinifyEnabled = false }
    }
}

val verifyNativeCore = tasks.register("verifyNativeCore") {
    doLast {
        val core = file("src/main/jniLibs/arm64-v8a/libaistudio2api.so")
        check(core.isFile && core.length() > 1_000_000) {
            "Build the real Android ARM64 Go/SpiderMonkey core first; placeholder APKs are disabled."
        }
        check(file("src/main/jniLibs/arm64-v8a/libc++_shared.so").isFile) {
            "The matching Android NDK C++ runtime must be bundled with the core."
        }
    }
}
tasks.matching { it.name.startsWith("package") && it.name.endsWith("Debug") || it.name == "packageRelease" }
    .configureEach { dependsOn(verifyNativeCore) }

dependencies { implementation("org.mozilla.geckoview:geckoview:159.0.20261102144711") }
