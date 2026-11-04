# Android ARM64 可行性审查

日期：2026-11-04。目标：vivo V2444A / Android 15 / ARM64。实际验收范围见 ANDROID_TEST_REPORT.md。

## 基线与结论

已通过 gh / GitHub API 验证现有 aishuishuishui277/AIStudio2API 是 Fork，parent/source 均为 Mag1cFall/AIStudio2API，用户有 push 权限。初始目录不是 Git 仓库，确认后才克隆；唯一 origin 是用户 Fork，没有重新 Fork 或向上游写入。

固定基线：a7afc31252dbb2543d79ac664eabf569148303bc。干净工作树创建 feature/android-standalone，main 保持原提交，不主动更新上游。

**原生编译、APK 装载、本地服务和 WebUI 已实机验证可行；Google 账号迁移和实际模型能力尚待验证。** 采用 Go c-shared / 小型 JNI / Kotlin 前台服务，复用原有 API、模型目录、SSE、服务装配和资源，不需要电脑常驻、Termux、Docker 或 VPS。

**当前基线没有纯 Go WAA。** config 仅接受 camoufox / spidermonkey；普通 WAA_BACKEND=go 无效，仅旧全量 .env 迁移会改为 spidermonkey。runtime_go.go 的历史名字不能证明纯 Go 实现；历史 9b694a6 已替换旧 JS 引擎。原型保留当前 WAA，使用 NDK 构建定制 SpiderMonkey，没有替换算法。

## 逐项核实

| 项目 | 状态 | 修改、证据与验证方法 |
| --- | --- | --- |
| Go 入口 / 生命周期 | 已适配，实机通过 | 独立 cmd/androidcore，复用 internal/app，避免 CLI os.Exit；JNI Start/Stop/Status；HTTP200→关闭监听→重启200，UI同步 |
| Android ARM64 / CGO | 已支持 | Go1.27.3 + NDK r30 / bionic 完整构建装载；强制 CGO=1，CGO=0 按预期失败，不能只凭 Go 交叉目标判断 |
| 定制 SpiderMonkey | 构建/基础 JS 通过；完整 proof 待验 | 原资产没有 Android；固定 e297ff009d3b9afee0e223c02ae3271ef24d51b6，仅 NDK r29→r30 补丁和 Mozilla JS Cargo workspace 过滤；定制符号、双context、Intl、Promise jobs、GC/关闭实机通过 |
| 动态库 / 16KB | 静态检查通过 | 链接 libjs_static.a / libjsrust.a，携带同一 NDK libc++_shared.so；两个 .so 为 AArch64、LOAD≥16KB、APK zipalign通过；vivo实际4096页，16KB设备未测 |
| 系统调用 / 可执行文件 | 关键路径已隔离 | 不在可写私有目录 exec 服务，使用 APK 原生库；Android拒绝 Camoufox 查找/登录，不下载桌面浏览器。设备装载和启停通过，所有媒体/QUIC分支未穷尽 |
| SQLite | 底层实机通过 | go-sqlite3经NDK编译；WAL、插入、关闭重开读取通过，真实账户账本待验 |
| 音频 / tokenizer | 编译通过；计数实机通过 | 完整API目标编译无额外缺失库；gemma3嵌入模型长文本计数通过；真实音频请求未测 |
| 浏览器指纹 | 本地生成通过 | 原BrowserForge生成Firefox152形状及时区；Google是否接受跨设备状态未测 |
| TLS / HTTP2 | 设备公网握手通过 | 保留原fhttp/utls客户端，经手机本地代理Google返回302/HTTP2，无Cookie；不是模型200 |
| HTTP3 / QUIC | 编译通过，运行待验 | uquic随核心编译；未验证UDP、HTTP3、网络切换 |
| 当前设备网络 | 需要本地代理配置 | VPN直连路径fake-IP连接拒绝；手机本地127.0.0.1:7890实测成功，仅配置本应用PROXY，不改VPN；不依赖电脑提供代理 |
| 路径 / 文件 / 时区 | 已适配 | 独立core进程，应用私有files/core，XDG/TMPDIR私有化，嵌入tzdata；已有.env不覆盖，无共享存储权限 |
| Windows 注册表 / Chrome | 已隔离 | 原平台文件隔离注册表实现；Android不调用Windows路径、不提取密钥；Windows安装与账户目录未改 |
| Cookie / 状态格式 | 导入已实现，迁移待验 | 复用StorageState cookies/origins及原导入API；不迁移Chrome OAuth/binding密钥/路径；解析成功不代表认证有效 |
| 凭据保护 | 合成测试通过，真实状态待验 | 私有目录，AES256-GCM，Keystore包装数据密钥，禁备份；随机nonce/往返/篡改和错误密钥拒绝实机通过；APK Keystore初始化和升级复用通过；关闭正文日志，不记录Cookie值 |
| 认证过期 | 原逻辑保留，真实验收待做 | 原CheckSession、Cookie过滤、AccountAuthRequired；需要真实过期和重新导入恢复测试 |
| Windows 账号导入 | 方案A已实现，未真实验证 | APK系统文档选择器导入本地JSON，原API校验、加密保存；用户自行选文件，不上传凭据 |
| Android 重新登录 | 方案B未实现 | 当前明确拒绝桌面登录。WebView/Custom Tabs不能直接等同于可取得Playground会话；GeckoView可研究，Google登录/状态转移/完整WAA均未验证，不声称成功率高 |
| Web 管理资源 | 实机通过 | 原锁文件npm ci/build与embed dist；WebView实际渲染账号/模型入口。Android构建隐藏桌面添加菜单、提示原生导入，普通Windows菜单保留 |
| Chat / SSE / 模型目录 | 原实现保留，业务待验 | 设备API离线回归26顶层/含子测试34通过；真实目录、生成、SSE必须账户验收，不能用mock替代 |
| Android15前台服务 | 已实现，部分通过 | specialUse、常驻通知及停止动作、START_NOT_STICKY；无WakeLock/无限重启；实机确认类型/通知/界面启停。通知点击、锁屏、回收待测，不承诺强杀存活 |
| 网络与文件权限 | 已配置，启动通过 | INTERNET/前台服务/通知；SAF文件选择；仅loopback2048/2049。同设备其他App可访问，应设置API key |
| 更新 / Windows兼容 | 已隔离，完整Windows回归未跑 | Android managed关闭桌面自更新；其他平台状态编码保持原路径；main与用户提交未覆盖 |

## 集成方式选择

gomobile/AAR是成熟选项，但不能免除本项目C++/Rust和生命周期问题。c-shared/JNI能以较小改动直接复用app装配，控制ARM64/16KB链接和错误返回。Kotlin前台服务在独立:core进程；不用Termux或桌面可执行文件启动方案。

## 主要剩余阻碍与验证

1. 用户在手机本地导入其Windows状态，验证会话、加密落盘/重启、Google完整WAA proof、认证过期与恢复。按要求在需要真实凭据处停止，不索取Cookie或密码。
2. 实际ListModels必须包含gemini-3.8-flash，再验证HTTP200、文本、SSE、多轮/长上下文；不硬编码模型凑验收。
3. SillyTavern模型选择、角色卡、10轮RP、30分钟、锁屏/网络切换、内存峰值与异常退出尚待实测。
4. Android原生登录需要独立实验，不能只删除报错。普通WebView的Google OAuth受限制；Custom Tabs不向嵌入应用开放Cookie。GeckoView扩展/本地消息仅提供候选集成方式，不能证明Google接受。

构建/配置/导入/回滚见 android/README.md；定制依赖准备见 android/native/README.md；仅交付debug原型，release等待实际验收。

## 官方资料（不代替项目实测）

- [Mozilla Android aarch64 SpiderMonkey](https://firefox-source-docs.mozilla.org/js/build.html#spidermonkey-on-android-aarch64)
- [Go Mobile](https://go.dev/wiki/Mobile)
- [前台服务类型](https://developer.android.com/develop/background-work/services/fgs/service-types)
- [Android15 dataSync限时](https://developer.android.com/about/versions/15/behavior-changes-15#datasync-timeout)
- [Android16KB页](https://developer.android.com/guide/practices/page-sizes)
- [Google WebView OAuth限制](https://developers.googleblog.com/en/upcoming-security-changes-to-googles-oauth-20-authorization-endpoint-in-embedded-webviews/)
- [Custom Tabs边界](https://chromium.googlesource.com/chromium/src/+/main/docs/security/custom-tabs-faq.md)
- [GeckoView扩展](https://firefox-source-docs.mozilla.org/mobile/android/geckoview/consumer/web-extensions.html)
