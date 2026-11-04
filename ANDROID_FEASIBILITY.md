# Android ARM64 原生移植可行性审查

审查日期：2026-11-04（Asia/Shanghai）。目标：vivo V2444A / Android 15 / ARM64。

## 基线与范围

- 已通过 `gh api repos/aishuishuishui277/AIStudio2API` 确认现有 Fork；`fork=true`，parent/source 均为 `Mag1cFall/AIStudio2API`，当前用户有 push 权限。
- origin：`https://github.com/aishuishuishui277/AIStudio2API.git`；未创建 Fork、未配置上游写入 remote。
- 克隆前当前目录不是 Git 仓库；克隆后 main 工作树干净。
- 固定基线：`a7afc31252dbb2543d79ac664eabf569148303bc`。不拉取上游新版本。
- 独立分支：`feature/android-standalone`（创建前远端不存在）。main 不修改。
- 本报告的“已有实现”表示源码证据，不表示 Android 实机 PASS。

## 初步结论

**有条件可行，但指定的“纯 Go WAA”路线在当前基线不存在。** 当前实现需要 Android NDK 和定制 SpiderMonkey，不能用 `CGO_ENABLED=0` 构建完整服务，也不能把 Linux ARM64 包当 Android 包。

`internal/config/config.go` 仅接受 `camoufox` / `spidermonkey`。旧全量 .env 的兼容迁移可把 `go` 改为 `spidermonkey`；普通配置中的 `go` 会被记录为无效并回退默认 Camoufox。`internal/aistudio/runtime_go.go` 文件名保留了历史名字，实际创建的是 `NewSpiderMonkeyWorker`。

`internal/waa/runtime.go` 直接使用 `internal/waa/mozjs`；后者 `import "C"`，链接定制 `libjs_static` 和 `libjsrust`。仓库历史 `9b694a6` 已替换旧 Go JS 引擎。不能靠改后端字符串恢复旧实现。

Mozilla 有 Android aarch64 的 SpiderMonkey 构建路径，但这只证明存在官方构建路线；项目定制版本的编译、链接、proof 和设备运行均须另外验证。当前先验证这项闸门，保留当前 WAA，不恢复历史版本、不替换整个实现。

## 逐项审查

| 项目 | 当前状态 | 证据 / 必要修改 | 验证方法 |
| --- | --- | --- | --- |
| Go 服务入口 | 需要修改 | `cmd/aistudio2api/main.go` 调用 `os.Exit(app.Run(...))`；不能直接嵌入 Android JVM | 新增独立 Android 入口，共用 `internal/app` 的装配和 HTTP handler；反复 Start/Stop |
| Android ARM64 编译 | 主要阻碍，验证中 | Go 支持目标不代表依赖可用；mozjs 为强制 CGO，现有链接规则是桌面规则 | `GOOS=android GOARCH=arm64 CGO_ENABLED=1`，NDK bionic 编译、ELF 检查及设备加载 |
| WAA_BACKEND=go | 当前基线不支持 | config 校验和历史替换均可复查 | 配置测试验证，不能用 alias 宣称恢复纯 Go |
| SpiderMonkey WAA | 主要阻碍，验证中 | release `v152.0.4-aistudio2api.1` 的 6 个资产仅 Windows/Linux/macOS，没有 Android；Linux ARM64 是 musl | 固定定制源码 `e297ff009d3b9afee0e223c02ae3271ef24d51b6`，NDK 构建；验证两个定制符号、JS API、proof |
| SQLite | 需要 NDK 编译 | `internal/sqlitedb` 用 `mattn/go-sqlite3`，其非 CGO stub 不能提供数据库服务 | Android 私有目录建库、WAL、停止重启、账本和对话库读写 |
| 音频原生代码 | 需要验证 | API 静态导入 gopus、oov/audio/resampler；不能因仅使用文本就忽略编译依赖 | 列出目标 CGO 文件；NDK 编译，保留原协议；文本/SSE 回归 |
| tokenizer | 已有嵌入实现，待目标验证 | `internal/aistudio/tokenizer.go` 嵌入 gemma3.model.zst，使用 go-sentencepiece | 目标编译和本地长文本计数 |
| 浏览器指纹 | 已有无浏览器路径，待设备验证 | `runtime_go.go` / `camoufoxnative.AccountFingerprint` 生成/保留 Firefox 宿主形状 | 导入指纹、VM 初始化、真实 Playground 请求；不能声称 Google 一定接受 |
| TLS / HTTP2 | 已有实现，待目标验证 | `transport_browser.go` 用 Mag1cFall/fhttp + utls；有自定义 HTTP2 pool/请求头顺序 | 逐依赖编译；设备信任库、TLS 握手、真实 HTTP2/SSE |
| HTTP3 / QUIC | 已有实现，待目标验证 | Mag1cFall/uquic；Android 虽匹配部分 linux tags，bionic 与 socket 行为仍需检查 | NDK/Go 目标构建；UDP 不可用时原有 HTTP2 路径和网络切换 |
| 网络可达性 | 外部必要条件，未验证 | 手机上的网络须能连接 Google；APK 不提供远端反代 | 设备实际网络测试，区分 DNS/TLS/认证/WAA/业务错误 |
| 路径与文件 | 需要修改 | app 大量使用相对 `.env`、auth/runtime；临时目录和时区数据不能依赖桌面系统 | 单独 core 进程固定 filesDir 下工作目录，TMPDIR 指向 cache，嵌入 tzdata；检查无共享存储权限 |
| Windows 注册表 / Chrome | 已有平台隔离，Android 需关闭入口 | chromeauth 的 Windows 实现分文件，非 Windows 返回不支持；ncrypt 设备绑定密钥不能直接移植 | Android 明确拒绝本机 Chrome 提取，不调用 Windows 路径 |
| 账号状态格式 | 已有可复用格式 | `StorageState{cookies,origins}`，外加 `aistudio2api` OAuth/source 扩展；账户目录另有配置、指纹、运行态 | 本地 JSON 导入测试 + 设备真实账号校验；格式解析不等于登录有效 |
| Cookie 过期 | 已有检测 | Cookie 到期过滤、Signer、CheckSession、AccountAuthRequired 与上游错误处理 | 人工过期状态 / 401；真实重新导入后恢复 |
| OAuth 续签 | 部分不可迁移 | Windows Chrome 的 wrapped binding key 依赖 Windows 密钥；私有 Chrome 路径在手机无效 | 移动导入仅取可携带 cookies/origins；不承诺桌面 OAuth 续签在 Android 有效 |
| Android 重新登录 | 尚未实现 | 现有登录仍通过 lazyLoginDriver 启动 Camoufox；Android WebView 不等价于受支持 Google 登录 | 本原型优先方案 A；方案 B 不能通过填密码/伪造 OAuth 授权解决 |
| Web 管理资源 | 已有嵌入机制，需要先构建 | `web/package-lock.json` 固定前端依赖；`internal/webui/embed.go` 嵌入 dist | npm ci/build，APK 断开电脑后打开页面；不能用占位页代替实际产物 |
| Chat Completions / SSE | 已有协议实现，待目标验收 | 原 API 与 upstream client 直接复用，不重写 | 单元回归 + 设备 HTTP 200、内容、SSE 终态、多轮/长上下文 |
| Android 15 前台服务 | 需要新增 | 用户启动的长期本地 API 不适合受六小时额度限制的 dataSync；考虑 specialUse 并声明用途 | API35 manifest、常驻通知、通知 Stop、启动限制、锁屏/系统回收；不承诺强杀存活 |
| 网络和文件权限 | 需要新增 | INTERNET；通知运行时权限；FOREGROUND_SERVICE / SPECIAL_USE；loopback 明文策略 | 不申请共享存储/账号/短信权限；仅 127.0.0.1 监听；同设备其他 App 仍可访问 loopback，支持 API key |
| 16KB / JNI | 需要验证 | Go + C++/Rust 共享库均需支持 Android 15 可能使用的 16KB 页；不可只 zipalign | readelf LOAD 对齐、APK zipalign、依赖库齐全，目标设备页大小与加载测试 |
| 凭据保护 | 需要新增 | upstream storage state 为私有明文 0600；Android 应禁备份，优先 Keystore 包装密钥保护落盘状态 | 合成 Cookie 测试不落明文、错误日志无值；设备 Keystore 和导入往返 |
| 更新/回滚 | 需要关闭桌面自更新 | `selfUpdater` 有 managed 模式；Android 由 APK 管理，不跟上游发行版 | managed 标识，无桌面包下载；保留旧 APK/数据，清数据/卸载会丢失 Keystore 密钥 |

## 集成方式比较

| 方式 | 结论 |
| --- | --- |
| gomobile bind / AAR | 官方成熟路径；仍要解决原生 C++/Rust 和生命周期，不能免除 WAA 构建 |
| Go c-shared + 小型 JNI | 可直接复用 app 包，桥接 Start/Stop/状态；依赖更少，便于固定 ARM64 与 linker 参数。先验证此路径 |
| 把可执行文件写入 filesDir 后 exec | 不选；Android 对可写应用目录执行文件有限制，错误地套用 Linux 启动逻辑风险高 |
| Termux / VPS / 桌面常驻反代 | 不采用，违背独立运行目标 |

## 实机验收闸门

当前 `adb devices -l` 没有设备；未读取或请求用户 Google 凭据，也未读取现有 Windows 运行目录中的账号状态。

以下全部为 **Not run**：APK 安装/启动、本地 API 与 WebUI、真实模型目录、gemini-3.8-flash HTTP 200/生成/SSE/多轮、SillyTavern 获取模型/角色卡/10轮 RP/长上下文、30分钟稳定性、锁屏、网络切换、内存、异常退出和账号迁移。

模型目录必须来自账户实际 ListModels，不能为了验收硬编码模型或把 mock HTTP 200 记为模型可用。完成构建只能交付 debug APK；release 须等待真实验收。

## 官方资料（能力与限制，不是项目实测结果）

- [Mozilla：SpiderMonkey Android aarch64 构建](https://firefox-source-docs.mozilla.org/js/build.html#spidermonkey-on-android-aarch64)
- [Go Mobile / AAR](https://go.dev/wiki/Mobile)
- [Android 前台服务类型](https://developer.android.com/develop/background-work/services/fgs/service-types)
- [Android 15 dataSync 限时](https://developer.android.com/about/versions/15/behavior-changes-15#datasync-timeout)
- [Android 16KB 页](https://developer.android.com/guide/practices/page-sizes)
- [本基线源码](https://github.com/aishuishuishui277/AIStudio2API/tree/a7afc31252dbb2543d79ac664eabf569148303bc)
- [定制 SpiderMonkey 版本与资产](https://github.com/Mag1cFall/firefox/releases/tag/v152.0.4-aistudio2api.1)
