#!/usr/bin/env bash
set -euo pipefail
: "${ANDROID_NDK_HOME:?Set ANDROID_NDK_HOME to the Linux NDK r30 directory}"
: "${A2A_SPIDERMONKEY_SOURCE:?Set A2A_SPIDERMONKEY_SOURCE to the pinned Firefox source checkout}"
: "${A2A_NATIVE_OUTPUT:?Set A2A_NATIVE_OUTPUT to an absolute build/output directory}"
repo=$(cd "$(dirname "$0")/../.." && pwd)
source_dir=$(cd "$A2A_SPIDERMONKEY_SOURCE" && pwd)
mkdir -p "$A2A_NATIVE_OUTPUT"
output=$(cd "$A2A_NATIVE_OUTPUT" && pwd)
obj=${A2A_NATIVE_OBJDIR:-$output/obj}
test "$(git -C "$source_dir" rev-parse HEAD)" = e297ff009d3b9afee0e223c02ae3271ef24d51b6
patch="$repo/android/native/spidermonkey-ndk-r30.patch"
if git -C "$source_dir" apply --reverse --check --unidiff-zero "$patch" 2>/dev/null; then
    :
else
    git -C "$source_dir" apply --check --unidiff-zero "$patch"
    git -C "$source_dir" apply --unidiff-zero "$patch"
fi
python3 "$repo/android/native/prepare-source.py" "$source_dir"
export MOZCONFIG="$output/mozconfig.android"
export MOZBUILD_STATE_PATH="$output/mozbuild"
toolchain="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64"
cat > "$MOZCONFIG" <<EOF
mk_add_options MOZ_OBJDIR=$obj
mk_add_options MOZ_MAKE_FLAGS=-j2
ac_add_options --enable-project=js
ac_add_options --host=x86_64-unknown-linux-gnu
ac_add_options --target=aarch64-linux-android
ac_add_options --with-android-ndk=$ANDROID_NDK_HOME
ac_add_options --disable-debug
ac_add_options --enable-optimize
ac_add_options --disable-tests
ac_add_options --disable-jemalloc
ac_add_options --disable-shared-js
ac_add_options --disable-warnings-as-errors
ac_add_options --with-libclang-path=${LIBCLANG_PATH:-/usr/lib/llvm-18/lib}
CC=$toolchain/bin/aarch64-linux-android35-clang
CXX=$toolchain/bin/aarch64-linux-android35-clang++
HOST_CC=clang
HOST_CXX=clang++
CFLAGS=-fPIC
CXXFLAGS=-fPIC
LDFLAGS=-Wl,-z,max-page-size=16384
EOF
cd "$source_dir"
./mach build
mkdir -p "$output/package/include" "$output/package/lib"
cp -RL "$obj/dist/include/." "$output/package/include/"
for library in libjs_static.a libjsrust.a; do
    built=$(find "$obj" -type f -name "$library" -print -quit)
    test -n "$built"
    cp "$built" "$output/package/lib/$library"
done
"$toolchain/bin/llvm-nm" -g --defined-only "$output/package/lib/libjs_static.a" > "$output/symbols.txt"
grep -q mozjs_set_camou_locale "$output/symbols.txt"
grep -q mozjs_delete_global_property "$output/symbols.txt"
sha256sum "$output/package/lib/"*.a > "$output/package/SHA256SUMS"
printf 'SPIDERMONKEY_ROOT=%s\n' "$output/package"
