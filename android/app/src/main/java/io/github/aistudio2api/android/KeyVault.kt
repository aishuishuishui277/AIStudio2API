package io.github.aistudio2api.android

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyStore
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

object KeyVault {
    private const val ALIAS = "aistudio2api-state-wrapper-v1"

    fun dataKey(context: Context): String {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        val key = if (store.containsAlias(ALIAS)) {
            store.getKey(ALIAS, null) as SecretKey
        } else {
            KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").apply {
                init(KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .setRandomizedEncryptionRequired(true).build())
            }.generateKey()
        }
        val preferences = context.getSharedPreferences("vault", Context.MODE_PRIVATE)
        val stored = preferences.getString("wrapped-data-key", null)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        val plain: ByteArray
        if (stored != null) {
            val payload = Base64.decode(stored, Base64.NO_WRAP)
            require(payload.size > 28) { "Invalid credential key envelope" }
            cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, payload.copyOfRange(0, 12)))
            plain = cipher.doFinal(payload.copyOfRange(12, payload.size))
        } else {
            check(!context.filesDir.resolve("core/auth").walkTopDown().any { it.name == "storage-state.json" }) {
                "Credential key is missing; clear app data and reimport account state"
            }
            plain = ByteArray(32).also { SecureRandom().nextBytes(it) }
            cipher.init(Cipher.ENCRYPT_MODE, key)
            check(preferences.edit().putString("wrapped-data-key",
                Base64.encodeToString(cipher.iv + cipher.doFinal(plain), Base64.NO_WRAP)).commit()) {
                "Could not save credential key"
            }
        }
        return Base64.encodeToString(plain, Base64.NO_WRAP).also { plain.fill(0) }
    }
}
