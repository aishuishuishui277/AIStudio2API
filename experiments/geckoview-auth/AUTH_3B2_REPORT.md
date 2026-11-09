# 3B-2：认证分类与用户主动更新

日期：2026-11-09（Asia/Shanghai）。设备：vivo V2444A / Android 15 / ARM64。

本轮已完成：同一账号候选验证、校验后取消保留旧密文、用户确认原子替换、新状态恢复、真实生成、force-stop 后自动恢复。没有自然过期的账号，因此不宣称真实过期恢复通过；错误分类及故障注入的离线测试与实机结果分开列出。

## 交付身份

- 分支：`feature/android-geckoview-auth-3b2`，从指定的 3B-1 源码 `d22c5d61a5b50ecb68cdcb94b4caf08be80c98a0` 创建独立 worktree。
- APK 源码提交：`af64f32ccd43eddd53624fcc7ad5ceac8e8a77ff`；后续文档提交不改变 APK 代码。
- 包名：`io.github.aistudio2api.authpoc`；versionCode 5；versionName `0.5.0-manual-auth-update-3b2`。
- APK：`AIStudio2API-GeckoView-Auth-3B2-arm64-debug.apk`，292334598 bytes。
- SHA256：`62080624e36f1f7bf499e26508b37da13701351887a61084ecdeb1a1952347c3`。
- 证书 SHA256：`01de4840d8ff72e3f7d150ec57ca2d5288e1cdf1e28efd7ed870133f00d0c783`，与既有实验包一致。
- 升级前确认设备 APK 是已验收 3B-1 哈希 `0a67e55f...d3745c4`；核对包名、签名后用 `adb install --no-streaming -r` 升级。最终安装包与交付件哈希一致。

GitHub API 再次确认目标为用户已有 Fork、parent 是 `Mag1cFall/AIStudio2API`。没有创建 Fork、同步上游、修改 main 或合并稳定分支。3A、3B-1、原 GeckoView PoC 及稳定分支的工作树保持原样。稳定 APK 仍为 `0.2.0-android-dual-stream`，更新时间仍是 2026-11-04 16:50:40。

## 错误分类

新增实验专用 `internal/authpoc` 策略和 Android 证据适配。分类是提示，不是删除账号、自动重新登录或读取 Cookie 的指令。

| 类别 | 使用的证据 |
|---|---|
| AUTH_REQUIRED | 原 PG/Chrome 认证器的明确 signed-out、`ErrAuthenticationRequired`、必要登录 Cookie 缺失，或固定账号不存在；不把一般未 Ready 当成失效 |
| NETWORK_FAILURE | 可识别网络错误；排除同样实现 net.Error 的 os.PathError |
| UPSTREAM_FORBIDDEN | HTTP 403 且无更明确证据；不会由此设置需要重新认证 |
| ACCOUNT_RESTRICTED | 原 RPC code 7 与明确 `Account not supported.`，或对应的 PG 错误格式 |
| QUOTA_EXCEEDED | HTTP 429、RPC code 8、已知 rate_limit_exceeded |
| WAA_FAILURE | 明确的 SpiderMonkey/WAA runtime 失败；已识别的网络原因优先 |
| UNKNOWN_ERROR | 证据不足、普通 401、存储/密钥问题等；固定 detail code 进一步说明，不能冒充登录失效 |

原 PG `ConfirmAuthenticationFailure` 在标记认证失效前还会检查官网会话；本轮没有替换其认证协议。原恢复器的 Chrome OAuth 续签分支依赖 OAuth 材料，本轮没有新增该材料或后台续期任务。

`CANCELLED` 是控制流程结果，不是认证失效。当前 JSON 包装仍显示 success=false/category=UNKNOWN_ERROR，但同时明确 `committed=false`、`needs_reauthentication=false`；不代表用户账号认证失败。

## UI 与授权边界

- **检查已保存账号**：仅 KeyVault → 原 codec → 固定 PG 目录；不启动 GeckoView、不读 Cookie。
- **重新登录 Google**：打开原 GeckoView 页面，保留 profile；用户自行操作，不抓取或提交密码、验证码、登录表单。
- **手动更新 PG 认证**：进入浏览器页面后，先由用户点“已登录，开始验证”才读取 Cookie；候选验证通过后，再由用户点“确认替换”才发布。
- 保留“一次生成”按钮。新版本第一次检查由用户触发；成功后才启用本版的冷启动 Ready 检查。自动恢复不自动生成，不读取 Cookie。

本轮用户截图显示已有 Playground 会话仍可使用；用户主动完成候选读取、取消和确认替换。没有强制退出既有 Google 会话，也没有观察到需要重新输入账号或完成挑战。因此“入口及现有会话确认”通过，不等同于一次全新 Google 密码登录或自然过期恢复。

## 发布与取消

1. 旧固定账号仍是 `files/pg-auth-3b1/auth/<原ID>/`，通过原 AccountStore 与 KeyVault 加载身份。恢复路径已完全移除 3A run 目录扫描。
2. 候选位于应用私有 `files/pg-auth-3b2/candidate-*`。复用原 Cookie 解析、ImportAccountState/VerifyAuthUser、WAA 和模型校验，账号 ID 不同立即拒绝。
3. 原 AES-GCM codec 加密候选；关闭候选 runtime 后再次解密加载验证。此时旧固定状态没有修改。
4. 等待用户二次确认。110 秒 UI/服务超时、120 秒 native 兜底超时；Activity 退出、Binder 死亡、取消或进程死亡都不能自动发布未确认候选。
5. 确认后核对旧密文摘要未改变，保存上一份完整密文为 `storage-state.json.previous`，同步备份与目录；新密文写同目录临时文件、fsync、再次比较旧状态，原子 rename 替换，再 fsync 父目录。
6. 新旧密文均用原 codec 重新读取，仅返回布尔结果；原账号 ID/config 不替换，不操作稳定 APK 数据。

加密由原 codec 完成；发布器只复制已验证密文，不另造加密协议。临时文件及 previous 都是私有密文。账号状态和请求认证头不进入 UI、logcat、Git、报告或公共存储；错误只返回类别、固定枚举和类型名。

取消与提交通过状态门串行化。用户点“确认替换”后 UI 明示短暂提交区不可取消：如果此时 Activity 消失或结果消息丢失，必须重新检查已保存状态，不能虚称取消成功。rename 后目录同步失败会报告 `committed=true` 与 durability unknown，不把已替换伪装成未替换。

异常终止时，磁盘目标是旧或新的一份完整文件；未提交候选不会在启动时扫描、导入或发布。异常中断可能留下加密 scratch，当前正常取消/提交都已清理。previous 不会被自动选作账号，不会自动覆盖 Go 后续轮换的新 Cookie。

## 离线测试（不是 Google 业务验收）

`go test -race -count=1 -timeout=60s -v ./internal/authpoc`：7 个顶层测试组通过，无 race 报告。含 10 类分类情形、取消/冲突/文件失败、无效封装拒绝、重复及并发提交、不同账号拒绝、备份恢复、三个真实测试子进程退出点。

崩溃点为 backup_synced、candidate_synced、renamed；重启读取目标时前两者得到旧完整文件，后者得到新完整文件，previous 保持旧完整文件。显式回滚算法恢复旧文件也通过。这些测试仅用明确标注的非敏感占位字节，验证状态机和文件事务，不证明 Google 认证或真实 AES-GCM 本身。

网络错误、未知 403、AUTH_REQUIRED、账号限制、配额、WAA、不同账号与中途崩溃的异常分支主要由离线测试/源码审查覆盖。没有为了造失败而破坏账号、修改 VPN 或伪造真实上游结果。

## 实机证据

| 项目 | 实际结果与边界 |
|---|---|
| 原持久化账号读取 | PASS：候选更新前由原 AccountStore/Keystore 成功加载原账号并核对身份。更新前单独的新版 Ready 结果未采集，不能用旧 3B-1 文件冒充 |
| 读取前取消 | 用户截图显示“未读取 Cookie、未修改旧状态”；当时没有 Cookie 审计记录或新 Go 结果 |
| 校验后取消 | PASS：用户亲自选择取消；`candidate_discarded=true`、`committed=false`、`old_ciphertext_verified_unchanged=true`；累计 Cookie API 调用 1 |
| 同一账号正式更新 | PASS：用户重新验证并确认替换；`same_account=true`、`committed=true`、`directory_synced=true` |
| 加密替换与旧备份 | PASS：`encrypted_state_reload=true`、`previous_encrypted_state_reload=true`；主文件和 previous 魔数均为 A2A-ANDROID-STATE-1；没有明文导出 |
| 新状态加载与模型 | PASS：固定目录恢复、Keystore、Ready、SpiderMonkey WAA，46 个真实模型，包含 gemini-3.8-flash |
| 新状态真实生成 | PASS：HTTP 200、有效 JSON、choices=1、finish_reason=stop、正文 OK（2 bytes），耗时 7.586s |
| force-stop 后恢复 | PASS：确认主/子进程退出后冷启动；PG PID 25161 → 27408；无按钮点击自动 Ready，耗时 6.549s；未加载主进程 Gecko libxul.so |
| 恢复不读 Cookie | PASS：更新结束累计 2，生成与冷启动后仍为 2，新增读取 0。两次读取均由用户主动确认 |
| 清理和隔离 | PASS：候选目录剩余 0；实验临时端口 37039 已释放；稳定 APK 版本/安装时间不变 |

真实生成仍使用 `Reply with exactly OK.`、max_tokens=2048、reasoning_effort=minimal、stream=false。没有自动重复生成，没有使用正式 2048。业务结果见 `committed-update-FG.json`、`new-state-generation-HI.json`、`cold-start-J.json`；取消证据见 `cancel-update-E.json`。

最初读取到的 `preexisting-3b1-result-not-3b2-evidence.json` 是旧版本遗留结果，已明确排除，不作为本轮 PASS。用户第一次取消在读取前确认框；解释后，用户完成了真正的校验后取消与正式提交。

## 修复轮次

共 1 轮失败后的源码修复：首次 APK 构建的 Kotlin 确认回调出现递归类型推断，给 Messenger 属性补充显式类型。修改仅涉及该声明。随后 clean assembleDebug lintDebug 成功（21s，47 tasks），lint 0 errors / 46 warnings。Go 构建、离线测试及上述实机主流程均通过；没有后续业务失败修复或无界重试。

## 构建、限制与停止点

仍使用既有 NDK r30 / SpiderMonkey Android 包、Go、JDK 21、SDK 37.2、AGP 9.4.1、Kotlin 2.4.21、Gradle 9.8.1、GeckoView 159.0.20261102144711。

准备原 `internal/webui/dist` 与 NDK/SpiderMonkey 后，在 Linux/WSL 设置 ANDROID_NDK_HOME、SPIDERMONKEY_ROOT，运行 `bash experiments/geckoview-auth/build-auth-probe.sh`。Windows 设置 JDK 21 JAVA_HOME 和实验工程 local.properties/sdk.dir，运行 `android/gradlew.bat -p experiments/geckoview-auth clean assembleDebug lintDebug`。升级必须同签名 `adb install -r`，不卸载、不清数据。

Not Run：自然失效账号的真实重新认证、真实不同账号替换、故意制造 Google 403/配额/账号限制、Android 更新中途强杀与突然断电、对已提交真实账号执行反向回滚。相关可离线验证的策略/事务已分别测试，不混称实机 PASS。旧备份可解密不保证其 Google 会话将来仍有效。

当前仍是有界实验探针；Ready 检查结束会关闭核心，不提供常驻 API。没有后台自动续期、多账号管理、稳定版合并、VPN/代理/TLS 修改。主流程完成后停止，后续工作由用户决定。
