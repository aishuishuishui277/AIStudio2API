# AIStudio2API Android 原生版

目标为 Android 15 ARM64。复用固定 Fork 基线的 Go 服务、Playground、OpenAI API、模型目录、SSE 和 Web 管理页面；通过 `c-shared` / JNI 嵌入 Kotlin 前台服务。运行时不需要电脑、Termux、Docker 或 VPS。

当前为 `0.2.0-android-dual-stream` debug 验证版（versionCode 2）。本设备已用现有迁移账号完成真实生成、双模式和隔离 SillyTavern 角色对话测试；本轮结果见仓库根目录 `ANDROID_DUAL_MODEL_TEST_REPORT.md`。早期 `ANDROID_TEST_REPORT.md` 保留为首个原型的历史记录。

## 安装与使用

1. 安装 debug APK，打开 AIStudio2API。允许通知，以便观察服务和从通知停止。
2. 点“启动本地服务”。默认 API 为 `http://127.0.0.1:2048/v1`，管理页面为 `http://127.0.0.1:2048`。
3. 导入账号状态（下节）。打开“账号状态”，检查认证；打开“模型目录”，检查账户真实可用模型。
4. 点“启动生成服务”。这一步可能访问 Google 并初始化 WAA。出现错误时查看管理页日志；不自动无限重启。
5. 在 SillyTavern 选择 OpenAI 兼容连接，填写上述 `/v1` 地址和管理页设置的 API key。获取模型列表，只有列表确实包含 `gemini-3.8-flash` 时才选择它。
6. 关闭界面不等于停止服务。点“停止本地服务”、通知“停止”或“停止并退出”来结束。系统强杀后需要手动启动。

手机网络本身必须能连接 Google。电脑仅用于构建、安装和可选诊断，不能充当 APK 的运行依赖。

本次 vivo 实测：当前 VPN 的直连路径对 Go 客户端返回 fake-IP 连接拒绝；管理设置配置 `PROXY=http://127.0.0.1:7890` 后，APK 自身“Google 网络诊断”返回 HTTP 302 / HTTP/2.0。这是手机本地代理，不是电脑代理；当前实例已保存此配置，新装默认留空，按设备实际端口配置。诊断不使用账号 Cookie，302 只证明公网连接，不能证明登录或模型可用。

## Windows 账号导入（方案 A）

- 从 Windows AIStudio2API 的 `auth/<邮箱>/storage-state.json` 复制你希望迁移的状态到手机；不要复制 Chrome User Data、Windows 密钥、数据库或整个 runtime。
- 点 Android 界面的“导入 Windows 账号状态 JSON”，通过系统文件选择器选中 JSON。文件只在设备内解析，复用现有 Cookie 导入接口；不会发往项目维护者或构建服务器。
- 导入接口只取可携带的 Google Cookie / origins，不导入 Windows Chrome OAuth refresh token、wrapped binding key、Chrome 路径。这些数据不能被假定能在 Android 续签。
- 文件解析和保存成功不等于会话有效。程序会继续使用原有上游校验、账号状态和错误处理；认证过期后重新导出/导入。无需把 Google 密码交给程序。
- 不复制 Windows 的完整账户指纹文件；默认使用现有引擎生成的持久指纹。本 vivo 已有迁移状态在升级后真实生成成功；其他账号或设备仍需各自验证。
- 导入副本在导入前由用户选择的存储位置保护；导入成功后可自行移除该副本。程序落盘的 storage-state 使用 AES-256-GCM 加密，数据密钥经 Android Keystore 包装；运行内存中仍需解密后使用。
- 不把账号文件、Cookie、`.env` 或完整请求正文提交到 Git。测试日志使用合成数据。

方案 B（Android 重新登录）尚未实现。现有 Camoufox 浏览器登录在 Android 返回明确不支持，不下载桌面浏览器；不能把普通 WebView 当作已验证的 Google 登录方案。

管理页“浏览器登录”的错误不是 Android 权限不足，无需授予额外系统权限。Android 构建的账号页提示返回 APK 主界面导入 JSON，隐藏桌面添加菜单。系统 Chrome / Custom Tabs 登录不能直接把 Cookie 提供给本应用；GeckoView 可作为后续实验，尚无本项目成功率数据。

## 配置与私有目录

所有数据在应用私有 `files/core` 目录：`.env`、`auth`、`runtime/logs`、SQLite 数据库、临时文件与缓存。Android 管理入口复用现有 Web 设置页。应用不申请共享存储权限，文件导入使用系统文档选择器。

Android 固定 `LISTEN_ADDR=127.0.0.1:2048`、`PREVIEW_LISTEN_ADDR=127.0.0.1:2049`、`WAA_BACKEND=spidermonkey`、私有账户目录、关闭请求正文记录与自动重试启动。这些项通过现有环境配置锁定机制呈现。`WAA_BACKEND=go` 在本基线已不是纯 Go 实现。

首次配置默认保留 1 个预热 Worker、最多 2 个活动 Worker、单账号并发 1、日志上限 50MB；只在 `.env` 不存在时创建，不覆盖已有配置。这些容量项可在管理页调整。

端口被占用时启动失败并显示错误，不静默改 API 端口。管理页现有预览服务保留自己的端口回退机制。若开启管理认证，先在内嵌管理页登录，原生导入和生成控制会复用该管理会话。

使用 `specialUse` 前台服务声明“用户启动的本地 API 服务”，持有常驻通知；无开机自启、WakeLock、无限拉起。国产系统电池策略、锁屏或进程回收仍可能停止连接。APK 关闭原有桌面在线更新，以固定基线为准。

## 按模型切换真流 / 假流

在 SillyTavern 的同一个 OpenAI 兼容连接中选择模型，无需切换全局开关：

| 客户端模型名 | stream=true | 实际上游目标 |
| --- | --- | --- |
| gemini-3.8-flash | 原有真流 | gemini-3.8-flash |
| gemini-3.8-flash-fake | 原有 FAKE_STREAM 完整结果后转 SSE | gemini-3.8-flash |

两者 `stream=false` 都返回普通 JSON。并发请求各自决定 Unary，不改全局状态。全局 `FAKE_STREAM=true` 仍可让所有模型使用假流；`false` 时仅 `FAKE_STREAM_MODELS` 列出的客户端原始名称启用假流。搜索/思考后缀沿用对应别名的模式。

配置使用现有逗号分隔格式，不把换行当成条目分隔符。例如：

```dotenv
FAKE_STREAM=false
MODEL_MAPPING=gemini-3.8-flash-fake:gemini-3.8-flash,gemini-2.5-pro-fake:gemini-2.5-pro,gemini-3-pro-preview-fake:gemini-3-pro-preview,gemini-3.1-flash-lite-fake:gemini-3.1-flash-lite
FAKE_STREAM_MODELS=gemini-3.8-flash-fake,gemini-2.5-pro-fake,gemini-3-pro-preview-fake,gemini-3.1-flash-lite-fake
```

Android 首次升级会补充主力文本模型的映射和假流名单，包括 2.5 Pro、3 Pro、3.1 Pro/Flash、3.8 Flash、latest、Gemma 等；只为实时真实目录存在的目标展示 `-fake`，不会把旧型号偷偷换成 latest 来宣称可用。当前账号没有普通 2.5 Pro / 3 Pro Preview 的独立目录项，因此不展示其假流别名。完整初始名单见 `internal/config/android_models.go`；可按账号目录自行扩展任意型号。

迁移仅修改 `MODEL_MAPPING` / `FAKE_STREAM_MODELS`，保留已有映射冲突和其他配置，在私有目录保存 `.env.before-model-modes-v1`；`.env.android-model-modes-v1` 标记防止反复补回。已明确配置 `FAKE_STREAM_MODELS`（包括空值）时不覆盖。管理设置页可编辑假流名单；保存后停止并重新启动生成服务。若要移除别名，请同时移除对应映射和名单项。

公开目录在 `/v1/models` 投影别名并继承目标能力；Web 管理的实时模型目录保留真实型号，虚拟别名不写入 Google 目录缓存。假流复用现有 Unary 和 SSE、心跳、取消及错误处理，可能输出多个紧邻的内容 chunk，不承诺逐字打字动画。

普通管理 WebView 已移除 `FLAG_SECURE`，允许截图和录屏；Keystore、AES-GCM、私有账号文件、Cookie 脱敏和 API Key 保护保持原样。

## 构建

固定依赖的具体准备步骤见 [native/README.md](native/README.md)。`build-core.sh` 会以 Android 专用构建变量重建 WebUI；普通 Windows 构建保留原菜单。

Go 核心要求 Android NDK / 定制 SpiderMonkey 的 Android 静态库，不能用 Linux ARM64 库替代。Linux/WSL 下先准备固定引擎源码、NDK r30、Rust 1.99.1 的 host/Android std、cbindgen 0.29.4、clang、libclang、Python 构建依赖。引擎源检出 `e297ff009d3b9afee0e223c02ae3271ef24d51b6`，构建脚本会应用 NDK 版本补丁，并复用 Mozilla 自带源包脚本的 Cargo workspace 过滤逻辑，仅构建 JS 相关成员。

```bash
cd web
npm ci --no-audit --no-fund
npm run build
cd ..
export ANDROID_NDK_HOME=/absolute/path/android-ndk-r30
export A2A_SPIDERMONKEY_SOURCE=/absolute/path/firefox-pinned-source
export A2A_NATIVE_OUTPUT=/absolute/path/android-native-build
bash android/native/build-spidermonkey.sh
export SPIDERMONKEY_ROOT="$A2A_NATIVE_OUTPUT/package"
bash android/native/build-core.sh
```

构建工具：Go 1.27.3、Android SDK 37（compileSdk）、targetSdk/minSdk 35、JDK 21、Gradle wrapper 9.8.1、AGP 9.4.1（内置 Kotlin，使用 2.4.21 插件工具链）。前端使用上游锁文件 `npm ci` 后 `npm run build`。

完整 native core 生成后，Windows 可以打包：

```powershell
$env:ANDROID_HOME = '你的 Android SDK 绝对路径'
.\android\gradlew.bat -p android :app:assembleDebug :app:lintDebug
```

打包任务要求真实 `android/app/src/main/jniLibs/arm64-v8a/libaistudio2api.so`，没有该文件时拒绝生成空壳 APK。debug 原始输出位于 `android/app/build/outputs/apk/debug/app-debug.apk`；交付时命名为 `AIStudio2API-Android-arm64-debug.apk`。

release 只在真实设备、账号、模型与 SillyTavern 验收后构建；签名密钥不进入源码。

## 回滚

- 不卸载 Windows 正式版，也不改其账号目录；Android 的安装与私有数据独立。
- 调试版保留上一份 APK。相同签名且版本号允许时可用 `adb install -r` 覆盖回到已保留的版本；降版本需按 Android 实际安装结果处理。
- 本轮升级前已保留 versionCode 1 的成功 APK。debug 包降级可用 `adb install -r -d <旧APK>`，先停止服务，不卸载或清数据。需要恢复原配置时，在停服后用 debug `run-as` 将私有 `.env.before-model-modes-v1` 复制回 `.env`；账号文件和 Keystore 不动。恢复前先保留自己的后续配置修改。
- 卸载或清除 Android 应用数据会删除私有状态及 Keystore 密钥，之后需要重新导入。Android 加密 storage-state 不应直接复制回 Windows。
- Git 回退使用独立分支的阶段提交，或检出固定基线到另一个目录；不重置 main，不强制推送。
