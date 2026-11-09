#!/usr/bin/env bash
set -euo pipefail
: "${ANDROID_NDK_HOME:?}" "${SPIDERMONKEY_ROOT:?}"
repo=$(cd "$(dirname "$0")/../.." && pwd)
cd "$repo"
test -f internal/webui/dist/index.html
export GOOS=android GOARCH=arm64 CGO_ENABLED=1
clang="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64"
export CC="$clang/bin/aarch64-linux-android35-clang" CXX="$clang/bin/aarch64-linux-android35-clang++"
export CGO_CXXFLAGS="-I$SPIDERMONKEY_ROOT/include" CGO_LDFLAGS="-L$SPIDERMONKEY_ROOT/lib"
out=experiments/geckoview-auth/app/src/main/jniLibs/arm64-v8a
mkdir -p "$out"
go build -mod=readonly -tags=authpoc -trimpath -buildmode=c-shared -ldflags='-s -w -extldflags=-Wl,-z,max-page-size=16384' -o "$out/libauthprobe.so" ./cmd/androidauthprobe
cp "$clang/sysroot/usr/lib/aarch64-linux-android/libc++_shared.so" "$out/"
