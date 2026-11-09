plugins { id("com.android.application") }
android {
    namespace = "io.github.aistudio2api.authpoc"
    compileSdk { version = release(37) { minorApiLevel = 2 } }
    defaultConfig {
        applicationId = "io.github.aistudio2api.authpoc"
        minSdk = 35
        targetSdk = 35
        versionCode = 2
        versionName = "0.2.0-cookie-metadata-poc"
        ndk { abiFilters += "arm64-v8a" }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    packaging { jniLibs { useLegacyPackaging = false } }
}
dependencies { implementation("org.mozilla.geckoview:geckoview:159.0.20261102144711") }
