package io.github.aistudio2api.android

import android.content.Context
import java.io.RandomAccessFile
import java.nio.channels.FileLock

/** Kernel releases the exclusive lease on process death. No credential data in this file. */
class ProcessLease private constructor(private val file: RandomAccessFile, private val lock: FileLock) : AutoCloseable {
    override fun close() { try { lock.release() } finally { file.close() } }
    companion object {
        fun acquire(context: Context): ProcessLease? {
            val file = RandomAccessFile(context.filesDir.resolve("pg-operation.lock"), "rw")
            return try { val lock=file.channel.tryLock(); if(lock==null){file.close();null}else ProcessLease(file,lock) }
            catch (_: Exception) { file.close();null }
        }
    }
}
