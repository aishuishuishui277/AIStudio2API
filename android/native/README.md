# 原生依赖构建

实际验证环境：Linux x86_64（WSL2 / Ubuntu 24.04）、Go 1.27.3、NDK r30、Rust 1.99.1（host + aarch64-linux-android）、cbindgen 0.29.4、clang/libclang 18、Python 3.12、JDK 21。原生目录使用无空格的 Linux 绝对路径。

依赖的 Firefox 仓库仅在本地检出固定版本，构建过程不会推送该仓库，也不会修改 AIStudio2API main。

```bash
sudo apt-get update
sudo apt-get install -y build-essential clang libclang-dev python3-venv pkg-config zlib1g-dev m4 unzip autoconf2.13 nasm

# 按你的工作区设置路径；准备 Go 1.27.3、NDK r30、Rust 1.99.1 与 cbindgen 0.29.4 后加入 PATH。
export ANDROID_NDK_HOME=/absolute/path/android-ndk-r30
export A2A_SPIDERMONKEY_SOURCE=/absolute/path/firefox-source
export A2A_NATIVE_OUTPUT=/absolute/path/android-native-build

git clone --filter=blob:none --sparse --depth 1 --branch v152.0.4-aistudio2api.1 \
  https://github.com/Mag1cFall/firefox.git "$A2A_SPIDERMONKEY_SOURCE"
git -C "$A2A_SPIDERMONKEY_SOURCE" sparse-checkout set \
  .cargo build config js mfbt nsprpub intl/bidi intl/components intl/icu intl/icu_capi intl/icu_segmenter_data \
  memory/build memory/mozalloc memory/replace modules/fdlibm modules/zlib modules/libpref/init mozglue python \
  third_party/fmt third_party/function2 third_party/python third_party/rust third_party/gemmology third_party/xsimd \
  testing/mozbase testing/manifest testing/mozharness testing/test testing/performance \
  testing/web-platform/mozilla testing/web-platform/tests/resources testing/web-platform/tests/streams \
  testing/web-platform/tests/tools testing/web-platform/meta/streams \
  toolkit/components/glean toolkit/mozapps/installer toolkit/crashreporter/tools \
  xpcom/geckoprocesstypes_generator xpcom/ds/tools tools/fuzzing tools/rb taskcluster layout/tools/reftest/reftest

# 在 AIStudio2API 仓库根目录执行：
bash android/native/build-spidermonkey.sh
export SPIDERMONKEY_ROOT="$A2A_NATIVE_OUTPUT/package"
(cd web && npm ci --no-audit --no-fund && npm run build)
bash android/native/build-core.sh
```

`build-spidermonkey.sh` 检查源提交 `e297ff009d3b9afee0e223c02ae3271ef24d51b6`，应用 NDK r30 一行补丁，并调用 Mozilla 源包脚本的原有 Cargo 过滤函数准备独立 JS workspace。没有替换 WAA。保留 JS shell 的构建，使必需的 `libjsrust.a` 也进入构建图；APK 只嵌入库，不执行该 shell。

脚本构建 2 个并发任务，导出头文件、`libjs_static.a`、`libjsrust.a`，并检查项目所需的两个定制符号。可选 `A2A_NATIVE_OBJDIR` 指定已经使用同一配置的对象目录以复用缓存。`LIBCLANG_PATH` 可覆盖 `/usr/lib/llvm-18/lib`。

`build-core.sh` 只构建 Android ARM64，共用原服务与 WebUI。它按原生库摘要更新 CGO 编译参数，避免更换外部静态库后误用 Go 缓存。随核心复制同一 NDK 的 `libc++_shared.so`；验证两个共享库的架构与 16KB LOAD 对齐。不要遗漏 C++ 运行库。

该脚本还会以 `VITE_ANDROID_STANDALONE=1` 重建 WebUI，让 Android 账号页提示返回 APK 主界面导入文件，隐藏不可用的桌面登录菜单。普通 Windows/Web 构建不设置此变量，保留原菜单。运行脚本前需安装前端依赖（`cd web && npm ci`）。

最后打包：

```powershell
$env:ANDROID_HOME = 'Android SDK 绝对路径'
.\android\gradlew.bat -p android :app:assembleDebug :app:lintDebug
```

构建检查缺少核心/C++ 运行库时会失败。安装：

```powershell
adb install -r .\android\app\build\outputs\apk\debug\app-debug.apk
```

不要在未完成真实账号、模型与 SillyTavern 验收前发布 release。保留同一签名密钥，才可覆盖升级并保留应用私有数据与 Keystore。
