# GeckoView → Android PG 认证兼容性：3A 实测

日期：2026-11-09（Asia/Shanghai）。设备：vivo V2444A / Android 15 / ARM64。结论：本机本账号的 3A 链路通过；未进入 3B。

## 版本隔离

从实验分支 `feature/android-geckoview-auth-poc` 的 `380720c6a165264a8672427e3f08239a500c99a3` 创建独立 worktree 和分支 `feature/android-geckoview-auth-3a`。原实验分支和稳定分支工作树均干净，未同步上游。

最终 APK 对应代码提交 `a59288b7be73be79c1ad5eddff299c10a1555613`。前一实现提交 `149b3c2`。包名保持 `io.github.aistudio2api.authpoc`，versionCode 3，versionName `0.3.0-auth-compatibility-3a`，ARM64。

APK SHA256：`dd2d1b21e94365dd12fba49649e4db2becc21f1f03974cd3820d71223e97b985`。签名证书 SHA256 与已安装实验包一致：`01de4840d8ff72e3f7d150ec57ca2d5288e1cdf1e28efd7ed870133f00d0c783`。均以 `adb install --no-streaming -r` 升级，没有卸载或清数据。最终安装 APK 哈希与交付件相同。

稳定分支保持 `58095c55e80aa2d415d172fe9c416a22feb1285d`，main 保持 `a7afc31252dbb2543d79ac664eabf569148303bc`。稳定 APK 仍 versionCode 2 / 0.2.0-android-dual-stream，更新时间仍为 2026-11-04 16:50:40。未访问其账号、Keystore、数据库、2048 API 或运行状态文件。

## 复用与最小适配

审查了 `internal/aistudio/auth.go`、`cookie_import.go`、`internal/setup/setup.go:importCookies`、`internal/app/admin.go:ImportAccountState` 以及 Android KeyVault/状态加密 codec。

实验调用原 `ParseCookieText`、Signer、runtimeAdmin.ImportAccountState、VerifyAuthUser、账号存储、模型同步、trackedService.Start、SpiderMonkey WAA 和 OpenAI handler；没有重新实现 Google 认证协议。新的 app helper 仅在 `android && authpoc` 构建标签启用，入口为 `cmd/androidauthprobe`。

发现原 StateCookie/导入器没有保留 hostOnly，Cookie 域匹配仅按域后缀。因此仅在集成分支增加 hostOnly 字段与精确主机匹配，解析器和后续 Set-Cookie 更新保留该标志；默认 false 保持旧状态兼容。增加专门格式回归测试，验证 host-only 不发送给子域、前导点 domain Cookie 仍匹配子域、unspecified 不变成 Lax。正式稳定分支没有此修改。

原解析器会忽略未知分区字段，因此在进入解析器之前双重检查：扩展拒绝非空 partitionKey / firstPartyDomain；Go 再次拒绝无法保真的分区状态。没有静默丢弃分区后继续验收。

| 字段 | 3A 处理 |
|---|---|
| name/value/domain/path | 用户点击后仅内部传递；domain 前导点保留；名称/值非空、path 以 / 开头 |
| hostOnly | 单独保存为布尔值，CookieHeader/CookieValue 按精确主机约束 |
| secure/httpOnly | 保留布尔值 |
| sameSite=no_restriction | 显式 None |
| sameSite=unspecified | 空字符串，保持原 Validate 允许的未指定语义 |
| expirationDate | 秒级数值转换为 expires，并检查已过期 |
| session=true | expires=-1，采用原格式 |
| partitionKey/firstPartyDomain | 非空时明确拒绝；当前真实样本无分区，分区登录兼容性未实测 |

## 用户触发与数据保护

三个真实实验都由用户在手机亲自点击“验证 PG 认证兼容性”触发并在聊天确认，代理没有代点，也没有自动再次请求 Cookie。

元数据按钮仍不访问 value；只有 auth 动作读取真实值。WebExtension 在既有 Google host 权限内获取当前登录会话，经 Native Messaging → Kotlin → 私有 Binder → `:pgprobe` 进程 → JNI 传递。没有写入 UI、控制台、logcat、Git、报告、公共存储或网络诊断文件。异常不透传原文，只映射固定错误类别；后续增加的 error_types 只包含 Go 类型名。

Go 进程使用独立 `files/pg-auth-3a/run-*`，各次目录相互独立。复用 KeyVault 的 Android Keystore 封装数据密钥和 AES-GCM 状态 codec，使用该实验包自己的 alias。账号识别邮箱不返回 UI/报告；PG 原有账号配置及加密认证状态留在私有实验目录。只读检查了文件魔数：3 个认证文件全部为 `A2A-ANDROID-STATE-1` 封装，没有读取或导出其中的明文值。

原始日志在实验进程禁用：i18n stdout/stderr、标准 log、slog 丢弃，requestRegistry 在 probe 中设置 silent；不创建完整管理服务器的账本和请求体采集。脱敏结果文件为 `files/pg-auth-3a-result.json`。敏感 byte buffer 在使用后清零，但 JVM/JS 字符串及框架内部复制由其内存管理决定，不声称所有副本都立即擦除。

## 网络与测试服务

应用 ID/UID 保持不变；只读核对 UID 10489 在当前 FlClash VPN 范围内。点击按钮还检查该应用活动网络是否为 VPN。不修改 VPN、节点、系统代理或 TLS 校验；Go 使用既有 VPN 路由，不额外配置代理。

测试 HTTP server 仅绑定 `127.0.0.1:0`，系统选择临时端口，并使用随机 API token。只放行 models 和 chat/completions 路径，不公开管理/账号导出路由。最终测试端口为 `127.0.0.1:42663`，没有占用正式 2048。

## 实际测试与修复轮次

| 实验 | 证据 | 根因假设/修改 | 验收结论 |
|---|---|---|---|
| 初版，用户第 1 次点击 | 23 个 Cookie 转换、核心接受、账户识别、加密保存成功；停在 model_catalog_and_waa，旧分类 NETWORK_FAILURE | 实验入口遗漏原 Android 入口的 XDG 私有缓存配置；WAA 租约依赖 userCacheRoot。旧 net.Error 判断还可能把文件错误误分类。原包无具体类型证据，不把精确根因描述为已完全证明 | 未通过 |
| 第 1 轮源码修复，用户第 2 次点击 | 补齐私有 XDG_CACHE_HOME/XDG_CONFIG_HOME，区分文件与网络错误并返回类型名；WAA 就绪、同步 46 模型、存在 3.8 Flash；HTTP 200，但正文为空 | 缓存修复后启动阻塞解除。测试预算 max_tokens=32 可能被思考耗尽；旧包没有 finish_reason，不能确定空正文的唯一原因 | 未通过，HTTP 200 单独不算成功 |
| 第 2 轮源码修复，用户第 3 次点击 | 复用此前成功请求的 max_tokens=2048 + reasoning_effort=minimal，补充 JSON/choices/finish_reason 诊断 | 实际返回 JSON 有效、choices=1、HTTP 200、finish_reason=stop、正文 OK（2 bytes） | 3A 通过，停止真实请求 |

共两轮有针对性的源码修复、三次用户手动触发。第 1 轮曾修正一个诊断代码 unused import 编译错误，未增加一次真实业务测试。没有第三轮业务修复或自动 UA/网络试错。

## 最终验收证据

| 步骤 | 结果 |
|---|---|
| Cookie 格式转换 | PASS，23 个 |
| PG 接受状态 | PASS，Signer 与导入流程通过 |
| Google 账户识别 | PASS，仅返回布尔结果，不披露邮箱 |
| 模型目录同步 | PASS，46 个，完整公开模型 ID 在本地 result-attempt3.json |
| gemini-3.8-flash | PASS，真实目录存在 |
| WAA | PASS，原 SpiderMonkey runtime 初始化成功 |
| 一次真实短文本生成 | PASS，请求“Reply with exactly OK.” |
| HTTP / 正文 | PASS，OpenAI 兼容端点 HTTP 200，实际 Google 链路返回 OK，finish_reason=stop |
| 构建 | PASS，最终 native build + Gradle clean assembleDebug lintDebug，17s，lint 0 errors / 21 warnings |
| 格式回归测试 | PASS，TestAuthProbeHostOnlyMapping；仅补充回归测试，不代替真实验收 |
| 资源释放 | PASS，结果返回后临时端口经独立 ADB forward 检查为 EOF/reset，无 HTTP 响应；移除了该临时 forward |

Android shell 的 ss 因 netlink 权限限制不能枚举端口，没有用空 ss 输出冒充释放证据。改用上述限定实验端口的检查。Go/JVM 进程可以被 Android 缓存，不要求 PID 消失。

没有抓取认证 Cookie、请求头或原始网络 trace；HTTP 200 为实际 OpenAI 兼容测试请求的响应状态，真实上游由既有 PG 实现完成，未使用 Mock。

## 交付与复现

源码构建先按原 android/native/README.md 准备固定 SpiderMonkey Android 包和 NDK r30。前端资源必须由同一基线构建（本次复用已验证稳定基线的 internal/webui/dist；不载入手机数据）。如从干净源码构建，先执行 npm --prefix web ci，再执行 VITE_ANDROID_STANDALONE=1 npm --prefix web run build。

在 Linux/WSL 设置 ANDROID_NDK_HOME、SPIDERMONKEY_ROOT 后运行 `bash experiments/geckoview-auth/build-auth-probe.sh`。随后配置实验工程 local.properties 的 sdk.dir，以 JDK 21 / Gradle 9.8.1 / AGP 9.4.1 / SDK 37.2 构建：

```powershell
.\android\gradlew.bat -p experiments/geckoview-auth clean assembleDebug lintDebug
adb install -r AIStudio2API-GeckoView-Auth-3A-arm64-debug.apk
```

Native helper 必须与 APK 一起打包；Go entry 由 authpoc 标签隔离。实验 APK 不提供自动登录管理或自动恢复验证任务，用户不点击时不会发起 3A 认证。

## 3B 建议（未实施）

本次仅证明当前账号、当前网络和当前 Google 状态下的兼容性，不证明长期免登录、续期、多个账号或多设备迁移可靠性。下一阶段需要用户确认后再设计：加密状态与浏览器 profile 的一致性、持久化状态冷启动重载、Cookie 轮换原子更新、过期检测与重新登录、用户取消/系统回收、账号切换和分区状态策略。保留正式账号数据回滚点；不得直接把实验成功等同于完整产品验收。

本轮不做 3B、不合并到稳定分支，三个实验认证目录保留在独立 PoC 私有目录中，没有自动删除用户数据。
