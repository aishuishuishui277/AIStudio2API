#!/usr/bin/env bash
set -euo pipefail
: "${ANDROID_NDK_HOME:?Set ANDROID_NDK_HOME to the Linux NDK r30 directory}"
: "${SPIDERMONKEY_ROOT:?Set SPIDERMONKEY_ROOT to the built Android include/lib package}"
repo=$(cd "$(dirname "$0")/../.." && pwd)
toolchain="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64"
export GOOS=android GOARCH=arm64 CGO_ENABLED=1
export CC="$toolchain/bin/aarch64-linux-android35-clang"
export CXX="$toolchain/bin/aarch64-linux-android35-clang++"
export CGO_LDFLAGS="-L$SPIDERMONKEY_ROOT/lib"
test -f "$SPIDERMONKEY_ROOT/include/jsapi.h"
test -f "$SPIDERMONKEY_ROOT/lib/libjs_static.a"
test -f "$SPIDERMONKEY_ROOT/lib/libjsrust.a"
native_stamp=$(sha256sum "$SPIDERMONKEY_ROOT/lib/libjs_static.a" "$SPIDERMONKEY_ROOT/lib/libjsrust.a" | sha256sum | cut -c1-16)
export CGO_CXXFLAGS="-I$SPIDERMONKEY_ROOT/include -DAISTUDIO_ANDROID_NATIVE_BUILD=0x$native_stamp"
cd "$repo"
test -f internal/webui/dist/index.html
out=android/app/src/main/jniLibs/arm64-v8a
mkdir -p "$out"
go build -mod=readonly -trimpath -buildmode=c-shared \
    -ldflags='-s -w -X github.com/Mag1cFall/AIStudio2API/internal/app.version=v0.1.0-android-prototype -extldflags=-Wl,-z,max-page-size=16384' \
    -o "$out/libaistudio2api.so" ./cmd/androidcore
cp "$toolchain/sysroot/usr/lib/aarch64-linux-android/libc++_shared.so" "$out/"
"$toolchain/bin/llvm-readelf" -W -h -l -d "$out/libaistudio2api.so" > "$out/elf-report.txt"
python3 - "$out/elf-report.txt" <<'PY'
import pathlib, sys
report = pathlib.Path(sys.argv[1]).read_text()
assert "AArch64" in report, "Core is not ARM64"
alignments = [int(line.split()[-1], 16) for line in report.splitlines() if line.lstrip().startswith("LOAD ")]
assert alignments and all(value >= 16384 for value in alignments), "Core LOAD segments are not 16KB aligned"
PY
"$toolchain/bin/llvm-readelf" -W -h -l -d "$out/libc++_shared.so" > "$out/libcxx-elf-report.txt"
python3 - "$out/libcxx-elf-report.txt" <<'PY'
import pathlib, sys
report = pathlib.Path(sys.argv[1]).read_text()
assert "AArch64" in report, "C++ runtime is not ARM64"
alignments = [int(line.split()[-1], 16) for line in report.splitlines() if line.lstrip().startswith("LOAD ")]
assert alignments and all(value >= 16384 for value in alignments), "C++ runtime is not 16KB aligned"
PY
sha256sum "$out/"*.so
