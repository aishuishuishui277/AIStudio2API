#include <jni.h>
#include "_cgo_export.h"
JNIEXPORT jstring JNICALL Java_io_github_aistudio2api_authpoc_AuthNative_verify(JNIEnv *e,jclass c,jstring dir,jstring key,jbyteArray payload){
 const char *d=(*e)->GetStringUTFChars(e,dir,0),*k=(*e)->GetStringUTFChars(e,key,0);
 jbyte *p=(*e)->GetByteArrayElements(e,payload,0);
 if(!d||!k||!p){if(d)(*e)->ReleaseStringUTFChars(e,dir,d);if(k)(*e)->ReleaseStringUTFChars(e,key,k);if(p)(*e)->ReleaseByteArrayElements(e,payload,p,JNI_ABORT);return NULL;}
 char *r=AuthProbe((char*)d,(char*)k,p,(*e)->GetArrayLength(e,payload));
 (*e)->ReleaseByteArrayElements(e,payload,p,JNI_ABORT);(*e)->ReleaseStringUTFChars(e,dir,d);(*e)->ReleaseStringUTFChars(e,key,k);
 jstring result=(*e)->NewStringUTF(e,r);AuthProbeFree(r);return result;
}

JNIEXPORT jstring JNICALL Java_io_github_aistudio2api_authpoc_AuthNative_restore(JNIEnv *e,jclass c,jstring dir,jstring key,jboolean generate){
 const char *d=(*e)->GetStringUTFChars(e,dir,0),*k=(*e)->GetStringUTFChars(e,key,0);
 if(!d||!k){if(d)(*e)->ReleaseStringUTFChars(e,dir,d);if(k)(*e)->ReleaseStringUTFChars(e,key,k);return NULL;}
 char *r=AuthRestore((char*)d,(char*)k,generate ? 1 : 0);
 (*e)->ReleaseStringUTFChars(e,dir,d);(*e)->ReleaseStringUTFChars(e,key,k);
 jstring result=(*e)->NewStringUTF(e,r);AuthProbeFree(r);return result;
}
