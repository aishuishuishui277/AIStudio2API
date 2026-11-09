# GeckoView 登录 PoC 实测记录

日期：2026-11-09（Asia/Shanghai）。设备：vivo V2444A / Android 15 / ARM64。

## 代码与产物

- Fork 经 GitHub API 确认：`aishuishuishui277/AIStudio2API`，parent `Mag1cFall/AIStudio2API`，push 权限 true。
- 原工作树干净，稳定分支本地/远程均为 `58095c55e80aa2d415d172fe9c416a22feb1285d`；`ff9b928` 是其祖先。没有回退已有双模型工作。
- 独立 worktree / 分支：`feature/android-geckoview-auth-poc`。
- APK 对应代码提交：`1e3f06e0b7ec36a2062af0e4c8f689a52739b8fb`。
- `AIStudio2API-GeckoView-Auth-PoC-arm64-debug.apk`：209068723 bytes。
- SHA256：`4eb3c8d0298ad78236e127b435d958915fb4b6594659ada48b19a9f61a58f446`。
- 包名 `io.github.aistudio2api.authpoc`，versionCode 1 / `0.1.0-geckoview-login-poc`。手机已安装 APK 的 SHA256 与交付件相同。

## 分级结果（不得合并为整体 PASS）

| 验收级别 | 结果 | 证据/限制 |
|---|---|---|
| 编译通过 | PASS | assembleDebug + lintDebug，BUILD SUCCESSFUL，39s；lint 0 errors / 7 warnings；签名 verify 通过；ARM64 |
| APK 安装和启动 | PASS | 独立包 `adb install --no-streaming` Success；am start Status ok，cold launch 1125ms |
| AI Studio 页面加载 | 未通过，网络阻塞 | 已发起 HTTPS 导航，但实机显示 category=3 / code=51；不是成功加载 |
| Google 登录成功 | Not Run | 尚未进入账号表单，没有输入账号、密码或验证码 |
| Cookie / nativeMessaging | Not Run | 按条件等待登录成功；只完成官方文档研究，没有安装采集扩展 |
| auth 可用于生成服务 | Not Run | 本轮未导出或导入真实凭据，也未连接稳定 Go 服务 |

## 网络根因证据与接管点

GeckoView 159 的 `WebRequestError.ERROR_NET_TIMEOUT = 0x33`，十进制 51，category 3 为 NETWORK，不是 SECURITY，也不是 Google 返回的嵌入式登录拒绝页面。

只读系统检查：PoC UID 为 10489；当前 FlClash VPN 的 UID 范围不包含 10489；系统 HTTP 代理为空。此结果与新应用没有纳入现有按应用代理规则一致。已请求用户将“AI Studio 登录 PoC”加入 FlClash 应用分流/访问控制，等待生效后再刷新一次。不自行切换节点、停止 VPN、修改全局路由、修改稳定版代理或设置 TLS 例外。

初始无账号超时画面保存在交付目录 `network-timeout-before-login.png`；网络记录只存 PoC UID 是否在 VPN 范围的布尔结果，不保存整份网络 dump。

网络恢复后，若出现 Google 账号表单，立即停止自动操作，由用户手动输入并完成验证。接管期间不截屏、不读取 UI/输入/页面数据、不抓浏览器日志。用户确认已到达 AI Studio 后再研究最小 Cookie 元数据扩展。若出现明确的“不安全浏览器/不允许登录”等错误，记录非敏感错误文本并停止，不做 UA 伪装重试。

## 构建修复记录

1. 初始依赖解析未使用开发机已有代理而阻塞；仅对本次 Gradle/SDK 下载进程配置已有代理，未改变系统或手机网络。
2. 初次构建暴露 Kotlin 元数据版本与默认编译器不兼容，以及 GeckoView 要求 compileSdk 37.2。PoC 配置 Kotlin 2.4.21 和 SDK 37.2，target/minSdk 保持 35。
3. 第二次构建定位界面字符串的换行转义语法错误，修正后第三次构建通过。共两轮源码修复；没有 Google 登录失败后的 UA 修改。

SDK manager 有旧 CLI 解析新 XML 的警告，SDK 已安装并由最终构建验证。Lint 警告保留在工件；没有隐藏或把失败构建记为成功。Gecko 预构建 native libraries 的 strip 提示不影响已验证的安装和启动。

## 稳定原型保护

- 修改范围仅 `experiments/geckoview-auth/`，不修改任何现有 Go、SpiderMonkey、Playground、Android 正式模块或 Web 管理页。
- 稳定工作树仍在 `feature/android-standalone`，HEAD 保持 `58095c5`，没有未提交更改；main 保持 `a7afc31252dbb2543d79ac664eabf569148303bc`。
- 稳定已安装包仍 versionCode 2 / `0.2.0-android-dual-stream`，lastUpdateTime 仍 `2026-11-04 16:50:40`。没有升级、卸载、清数据或停止它。
- PoC 使用独立 `/data/user/0/io.github.aistudio2api.authpoc`；没有读取或复制稳定账号/Keystore，未将真实凭据写入 Git、日志或交付件。

## 当前可行性判定

原生 ARM64 GeckoView 的打包、安装、启动已证实可行。Google 是否允许此默认 GeckoView 登录尚无实测结论；当前阻碍为按应用 VPN 路由，不应判定核心方案不可行，也不能宣称登录成功。

下一阶段的必要门槛依次是：恢复该独立应用的联网 → 用户完成 Google 登录并到达 AI Studio → 白名单、无值 Cookie 元数据和 nativeMessaging 实验 → 后续另行授权的真实 auth 兼容验证。本轮没有达到后三级，暂不建议合并稳定应用。
