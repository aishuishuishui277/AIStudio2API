# Android GeckoView 登录 PoC

历史第二阶段已完成：用户手动登录及 Playground 生成成功；在 vivo V2444A 上实测内置扩展、Native Messaging 和 23 个 Google Cookie 的无值元数据读取。升级、冷启动后均保留 Playground 会话。详见 [第二阶段报告](COOKIE_METADATA_REPORT.md)。第一阶段 `TEST_REPORT.md` 的网络阻塞结论是历史记录，已被新实测更新。第三阶段 3A 已在独立集成分支通过：用户主动触发后，复用 PG 导入流程识别账号、同步 46 个模型，并完成 gemini-3.8-flash 单次真实生成（HTTP 200 / OK）。详见 [3A 实测与构建报告](AUTH_3A_REPORT.md)。不进入 3B。

## 隔离和验收边界

基线 `58095c55e80aa2d415d172fe9c416a22feb1285d`，包含 `ff9b928` 及后续双模型功能。当前集成分支 `feature/android-geckoview-auth-3a` 从原实验分支的 `380720c` 创建，独立 worktree，不合并稳定分支。

独立 ID `io.github.aistudio2api.authpoc`，独立 UID 和 Gecko 私有 profile。应用声明 INTERNET；Gecko 依赖合并 NETWORK_STATE、WAKE_LOCK、MODIFY_AUDIO_SETTINGS 等普通权限，没有申请账号、文件或运行时危险权限。3A 包含隔离的 Go/SpiderMonkey 探针进程，不访问稳定应用的 API、文件或 Keystore。关闭 Android backup。Gecko Cookie 数据库属于浏览器私有数据；本轮没有额外实现数据库 Keystore 加密，不能声称与正式账号存储同等保护。

默认 Gecko UA、正常 TLS、无证书例外；关闭远程调试、控制台转发和 debug logging。不记录 URL 参数、标题、HTML、密码或 Cookie 值。3A 仅在用户亲自点击“验证 PG 认证兼容性”后在应用内部传递真实 Cookie；认证状态使用独立私有目录与 Keystore/AES-GCM 保存，只展示脱敏结果。第二阶段只有手动点击“检查登录 Cookie”才读取元数据，结果显示在对话框，并保存在应用私有 `files/cookie-metadata.json`（不含值）。Google 账号输入和验证仍由用户手动完成；输入期间不自动截屏、dump UI、抓完整 logcat 或复制 profile。

四级验收独立记录：编译通过、页面加载、Google 登录成功、auth 可用于生成服务。前一级成功不能替代后一级。本轮不向正式生成服务导入凭据。Google 明确拒绝嵌入式登录时停止；最多三轮修复，不反复切换 UA。

## 现有认证代码

- `android/.../MainActivity.kt:onActivityResult` 从文件选择器读取用户选定的 JSON（2 MiB 上限），发送到本机 `/api/accounts/import/cookies`。这不是 Android Google 登录。
- `internal/app/admin.go:ImportAccountState` 验证账号/状态及 Signer；可解析并不等于 Google 接受。
- `internal/aistudio/auth.go:WriteStorageState` 校验并原子保存；`auth_codec_android.go` 使用 AES-GCM，`KeyVault.kt` 用 Android Keystore 封装数据密钥。3A 复用原加密实现，并对 Cookie 导入和域匹配增加 hostOnly 保真处理，详见 3A 报告。
- Gecko Cookie 的 SameSite、session/expiry、partitionKey、firstPartyDomain、storeId 需要逐项分析，不能直接当成 Playwright storage-state。Chrome OAuth/设备绑定数据也不能假设从 Gecko Cookie 自动得到。

## 官方 v0.3.3 研究

只下载隔离参考源码，没有合并上游。tag commit：`718ea76f1c0d5a981c6368f936ec29c1acc5429e`。

- [login.go](https://github.com/Mag1cFall/AIStudio2API/blob/v0.3.3/internal/camoufoxnative/login.go)：启动 Camoufox，通过 BiDi 检查 AI Studio prompt readiness，读取账号标识、Drive 授权，再导出 `storage.getCookies` 和各 origin 的 localStorage。
- [liveview.go](https://github.com/Mag1cFall/AIStudio2API/blob/v0.3.3/internal/camoufoxnative/liveview.go)：`browsingContext.captureScreenshot` / `input.performActions`。
- [login_view.go](https://github.com/Mag1cFall/AIStudio2API/blob/v0.3.3/internal/api/login_view.go) 与 [LoginViewer.vue](https://github.com/Mag1cFall/AIStudio2API/blob/v0.3.3/web/src/components/LoginViewer.vue)：WebSocket 画面和输入转发。
- [Termux issue #105](https://github.com/Mag1cFall/AIStudio2API/issues/105)：Debian/proot、GTK/X11/Xvfb、Linux ARM64 包。用户成功反馈不能当作 Android GeckoView 成功证据。

## Mozilla 文档与条件性扩展实验

固定官方稳定 GeckoView `159.0.20261102144711`，AGP 9.4.1 / Kotlin 2.4.21 / Gradle 9.8.1 / JDK 21，compileSdk 37.2 / targetSdk 35。

- [Getting Started](https://firefox-source-docs.mozilla.org/mobile/android/geckoview/consumer/geckoview-quick-start.html)：官方 Maven 和 Runtime/Session/View。
- [WebExtensions](https://firefox-source-docs.mozilla.org/mobile/android/geckoview/consumer/web-extensions.html)：APK assets 内置扩展和 `ensureBuiltIn`；后台脚本的 `geckoViewAddons` + `nativeMessaging` 与原生 MessageDelegate 通信，不需要内容脚本权限。
- [cookies.getAll](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/cookies/getAll)：cookies 和对应 host 权限。文档支持不等于实机已验证。

第二阶段扩展已经实现并完成真实设备测试。限定三个 Google host 权限、手动触发一次扫描，在扩展内投影为无值白名单再发送消息；接收侧再次校验。expiry/partition 仅报告存在性，不传输具体内容。元数据动作禁止发送 value、原始 Cookie 对象、localStorage 或任意异常对象。3A auth 动作单独受用户点击控制，仅内部传递经过字段适配的认证状态。另有合成 getter 防护检查作为补充，不替代实机结果。Cookie API 成功仍不代表 auth 可用。

## 构建

3A 必须先按 [3A 报告](AUTH_3A_REPORT.md) 构建前端资源及 `build-auth-probe.sh` 原生库，再执行以下 Gradle 命令。

设置 JDK 21 的 JAVA_HOME 和 SDK 的 ANDROID_HOME，或在本目录 local.properties 配置 sdk.dir（已忽略）。从仓库根目录执行：

```powershell
.\android\gradlew.bat -p experiments/geckoview-auth assembleDebug lintDebug --console=plain
adb install -r experiments/geckoview-auth/app/build/outputs/apk/debug/app-debug.apk
adb shell am start -n io.github.aistudio2api.authpoc/.MainActivity
```

开发机如需代理，仅为构建进程配置 JVM http(s).proxyHost / proxyPort，不提交私人代理、不改变手机 VPN。手机运行不依赖电脑、Termux 或远程浏览器。稳定 APK 不升级、不清数据。退出 PoC 关闭会话，浏览器私有 profile 保留。
