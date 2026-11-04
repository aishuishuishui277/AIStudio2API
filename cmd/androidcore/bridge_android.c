#include <jni.h>
#include "_cgo_export.h"

JNIEXPORT jstring JNICALL Java_io_github_aistudio2api_android_NativeCore_start(
    JNIEnv *env, jclass klass, jstring directory, jstring key) {
  const char *dir = (*env)->GetStringUTFChars(env, directory, 0);
  const char *secret = (*env)->GetStringUTFChars(env, key, 0);
  if (!dir || !secret) {
    if (dir) (*env)->ReleaseStringUTFChars(env, directory, dir);
    if (secret) (*env)->ReleaseStringUTFChars(env, key, secret);
    return NULL;
  }
  char *message = CoreStart((char *)dir, (char *)secret);
  (*env)->ReleaseStringUTFChars(env, directory, dir);
  (*env)->ReleaseStringUTFChars(env, key, secret);
  jstring result = (*env)->NewStringUTF(env, message);
  CoreFree(message);
  return result;
}

JNIEXPORT jstring JNICALL Java_io_github_aistudio2api_android_NativeCore_stop(
    JNIEnv *env, jclass klass) {
  char *message = CoreStop();
  jstring result = (*env)->NewStringUTF(env, message);
  CoreFree(message);
  return result;
}

JNIEXPORT jstring JNICALL Java_io_github_aistudio2api_android_NativeCore_status(
    JNIEnv *env, jclass klass) {
  char *message = CoreStatus();
  jstring result = (*env)->NewStringUTF(env, message);
  CoreFree(message);
  return result;
}

JNIEXPORT jstring JNICALL Java_io_github_aistudio2api_android_NativeCore_probe(
    JNIEnv *env, jclass klass) {
  char *message = CoreProbe();
  jstring result = (*env)->NewStringUTF(env, message);
  CoreFree(message);
  return result;
}
