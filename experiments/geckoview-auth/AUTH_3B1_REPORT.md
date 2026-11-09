# 3B-1 加密账号持久化与冷启动恢复

日期：2026-11-09，Asia/Shanghai。设备：vivo V2444A / Android 15 / ARM64。

结论：本机已保存账号的迁移、Keystore 恢复、独立 PG 进程重启、整包 force-stop 后自动恢复，以及冷启动后的真实生成均已验证。第一次生成曾返回 403，第二次返回 200 / OK；不声称所有请求都成功或 403 根因已解决。停止于 3B-1。

## 版本与构建

- Fork 经 GitHub API 确认为 `aishuishuishui277/AIStudio2API`，parent 为 `Mag1cFall/AIStudio2API`。
- 独立分支 `feature/android-geckoview-auth-3b1` 严格从指定的 `a59288b7be73be79c1ad5eddff299c10a1555613` 创建。
- APK 代码提交：`d22c5d61a5b50ecb68cdcb94b4caf08be80c98a0`。后续文档提交不改变 APK 源码。
- 包名 `io.github.aistudio2api.authpoc`，versionCode 4，versionName `0.4.0-saved-auth-3b1`。
- APK：`AIStudio2API-GeckoView-Auth-3B1-arm64-debug.apk`，292301822 bytes。
- SHA256：`0a67e55f9bfddcd251cee720f6618f7e9d87c18b8dc4ce6f7e1e16d25d3745c4`。
- 签名证书 SHA256：`01de4840d8ff72e3f7d150ec57ca2d5288e1cdf1e28efd7ed870133f00d0c783`，与原实验包相同。
- 升级前核对设备上的 3A APK 哈希与已验收交付件一致；每次升级核对签名，全部用 `adb install --no-streaming -r`，不卸载、不清数据。最终设备 base.apk 哈希与交付件一致。
- Go Android ARM64 native build 通过；最终 Gradle `clean assembleDebug lintDebug` 17s / 47 tasks；lint 0 errors / 36 warnings（保留完整报告）。中间增量 APK 体积膨胀，交付前已 clean 构建并重新安装同源码产物。

main 保持 `a7afc31252dbb2543d79ac664eabf569148303bc`；稳定分支保持 `58095c55e80aa2d415d172fe9c416a22feb1285d`；3A 分支保持其文档提交 `3924522f46b8a5eabb9eeb1024e8dc11009d577b`；原 GeckoView PoC 分支保持 `380720c6a165264a8672427e3f08239a500c99a3`。各原工作树未改动。稳定 APK 仍是 `0.2.0-android-dual-stream`，lastUpdateTime 仍为 `2026-11-04 16:50:40`。

## 固定目录与一次性迁移

原 3A 在三个 `files/pg-auth-3a/run-*` 目录中留下了加密状态。本轮未导出、删除或覆盖这些原件。

首次恢复要求原脱敏结果 `pg-auth-3a-result.json` 为 success / complete，然后只在该应用私有范围内选取修改时间最新的 3A storage-state。文件晚于成功结果时间、时间并列或数据无效时拒绝迁移。旧 3A 结果没有显式 run ID，因此选择依据是成功结果与文件修改时间，而不是新增的历史数据库。

新固定目录为 `files/pg-auth-3b1/auth/<原账号ID>/`，复用原 `account.json` 配置及账号 ID。使用原 `AccountStore.Load` / `LoadStorageState` / `WriteStorageState`，在进程内解密、重新加密至私有暂存目录；验证加载成功和身份一致，sync 后原子 rename 发布整个 auth 目录。原文件迁移前后密文摘要一致，仅返回布尔结果，不输出摘要或账号路径。

固定 auth 目录一旦存在，后续只加载它；即使它失效或损坏，也不会重新扫描旧 run 目录或回退到 GeckoView。迁移失败不替换现有有效目录；暂存清理仅针对本次新建的 scratch 目录。

Keystore alias 继续使用 `aistudio2api-authprobe-wrapper-v1`，同一个包、UID、`vault` 密钥封装偏好及 AES-GCM codec。恢复入口要求 alias 和 wrapped-data-key 已存在，禁止在恢复时生成替代密钥。没有读取稳定 APK 的私有数据。

## 操作与生命周期

新默认入口 `SavedAccountActivity` 是纯原生页面，不实例化 GeckoRuntime、GeckoSession 或 CookieInspector。

1. **启动已保存的 PG 账号**：首次必要迁移，之后直接加载固定目录、初始化 WAA、检查模型与 Ready。
2. **验证已保存账号生成（一次）**：直接使用固定目录完成相同加载流程，再发起一个指定短文本请求。
3. **打开原 GeckoView 登录 PoC**：保留原页面和登录 profile；本轮测试没有点击它。

第一次 Ready 检查成功后保存启动偏好；冷启动自动恢复并检查 Ready，不自动生成。生成失败不再关闭冷启动恢复偏好。恢复失败会关闭自动恢复，显示错误，等待手动处理；无循环重启或浏览器回退。

本轮是有界探针：每次完成后关闭 PG runtime 和临时 HTTP listener，UI 的 Ready 是此次检查结果，不代表常驻 API 服务。没有新增长期前台服务，也没有占用正式 2048。

恢复 Binder 请求只携带操作码，不携带 Cookie。JNI 恢复入口不调用 ParseCookieText / ImportAccountState；原 3A 导入入口保留，但只写原 3A 目录，与持久目录隔离。原 browser Cookie API 前增加无值审计事件，计数从本版开始；本轮全程为 0。冷启动还检查了主进程未加载 Gecko `libxul.so`，并核对 UI 的 automatic_restore=true，而不是只依赖一个硬编码状态字段。

## 真实测试记录

所有业务测试使用真实 Google 上游。没有 Mock、Windows auth 文件、新 Google 登录或 Cookie 重新获取。未改 FlClash、代理、TLS 或节点。启动前仍检查当前应用使用 VPN。

| 记录 | 操作 | 耗时 | 结果 |
|---|---|---:|---|
| A | 当前 APK 从最后一次 3A 加密状态迁移并恢复 | 13.829s | PASS；identity 保留、Keystore 恢复、WAA、Ready、46 模型；原件未变 |
| B | 仅 TERM 已确认的 PG 子进程，再点恢复 | 9.121s | PASS；PID 31418 → 31878；migration=false；固定目录加载 |
| C | 整包 force-stop，确认主/子进程均消失，再冷启动 | 5.519s | PASS；新主进程、新 PG PID 32328；未点恢复按钮，自动 Ready |
| D | C 后的一次指定生成 | 5.538s | FAIL；HTTP 403；完整脱敏失败记录保留 |
| E | 第 1 轮修正后的最终包直接恢复 | 3.942s | PASS；PID 6974；固定目录、Keystore、Ready |
| F | 最终包再次 force-stop 后冷启动 | 6.027s | PASS；PG PID 7282；无点击自动恢复；无 Gecko 库加载 |
| G | F 后的一次指定生成 | 7.571s | PASS；HTTP 200；JSON 有效；choices=1；finish_reason=stop；正文 OK，2 bytes |

共五次 Ready 检查、两次生成请求；没有后台自动重发生成按钮。各记录的 `geckoview_cookie_reads=0`，全部恢复到同一固定账号目录，目录包含 `gemini-3.8-flash`、共 46 个模型。最终结果见 `generation-G-final-after-cold-start.json`；完整模型 ID 列表仅为公开模型名，无账号身份。

指定请求始终是 `Reply with exactly OK.`，model=`gemini-3.8-flash`，stream=false，max_tokens=2048，reasoning_effort=minimal。只有 HTTP 200、有效 JSON、非空 choices、stop 和非空正文才能通过；空正文、length 或非 200 不通过。

### 修正轮次与未确定因素

仅一轮源码修正，未达到三轮上限。

- 实际失败：D 的 Ready、模型和 WAA 均通过，随后生成返回 HTTP 403。初版没有保存细分错误，无法证明其具体原因。
- 最小修改：加入固定白名单错误诊断（地区/账号限制、权限拒绝、认证、Bot challenge、Drive、生成阶段、OAuth、配额）；不记录上游原文或任意字符串。同时修正“生成失败会关闭冷启动恢复偏好”的 UI 状态问题。
- 未修改认证、Cookie 来源、WAA、请求协议、VPN、代理或重试策略。
- 实际证据：修正版最终 G 返回 200 / OK，使用的是同一持久化状态且 Cookie 读取为 0。
- 结论：最终功能验收通过，但诊断改动不能被描述成已修复上游 403。该次 403 原因未定，不排除以后再次发生；没有把失败全归因于 Cookie，也不保证成功率。

## Set-Cookie 与持久化一致性

复核原 `AccountLease.MergeSetCookieHeaders`：持有 storage 锁后重新加载最新磁盘状态，合并 Set-Cookie，先 WriteStorageState 成功，再更新内存状态。原 WriteStorageState 在创建临时文件前完成 AES-GCM 编码；atomicWriteFile 写同目录临时密文、sync、close、rename。未更换加密算法，也未建立第二套 Google 认证协议。

实机证据：

- A、D、G 的 Cookie 状态摘要在内存比较中发生变化，仅输出 `cookie_state_changed=true`，不输出 Cookie、Header 或摘要。G 后重新加载通过，封装仍为 `A2A-ANDROID-STATE-1`。
- 四个实际 storage-state 文件（旧 3 个、新 1 个）全部仅检查魔数，均为加密封装；未读取明文到电脑。
- 每次检查在新建私有 scratch 目录用真实状态创建加密副本，然后将该 scratch 目录设为只读，触发真实文件权限写失败；验证失败前后密文相同且仍能用原 codec 加载。`failed_write_preserves_previous=true`。不对有效账号原件做故障注入。
- 旧 GeckoView 入口只更新浏览器或旧 3A run 目录，恢复入口只读固定目录，不会用旧浏览器 Cookie 覆盖 PG 新状态。
- 最终 `127.0.0.1:40401` 已关闭：限定端口的临时 ADB forward 得到 EOF/reset、无 HTTP 响应；该 forward 随后移除。

边界：普通写入失败的旧文件保全已实测；突然断电/存储硬件故障未测。原文件替换实现未额外 fsync 父目录，不能宣称断电持久性已完整证明。JVM/JS/Go 内存副本由运行时管理，不声称所有副本立即擦除。

## 构建与安装

复用原 Android toolchain：NDK r30、已有 SpiderMonkey Android 包、Go、JDK 21、SDK 37.2、AGP 9.4.1、Kotlin 2.4.21、Gradle 9.8.1；GeckoView 保持 159.0.20261102144711。

1. 按 `android/native/README.md` 准备 NDK 和 SpiderMonkey。
2. 准备同基线的 `internal/webui/dist`（本次复用既有构建；干净构建先 `npm --prefix web ci`，再在 Linux/WSL 执行 `VITE_ANDROID_STANDALONE=1 npm --prefix web run build`）。
3. Linux/WSL 设置 ANDROID_NDK_HOME、SPIDERMONKEY_ROOT，执行 `bash experiments/geckoview-auth/build-auth-probe.sh`。
4. 设置 JDK 21 JAVA_HOME、Gradle 缓存和实验工程 local.properties 的 sdk.dir；执行 `android/gradlew.bat -p experiments/geckoview-auth clean assembleDebug lintDebug`。
5. 核对原实验签名后 `adb install -r AIStudio2API-GeckoView-Auth-3B1-arm64-debug.apk`。

不要卸载、清数据或随意更换签名，否则可能失去原 Keystore/profile。出现问题时保留私有目录，使用同签名、递增 versionCode 的修复包；不提供清数据“回滚”。稳定 APK 始终独立，未升级。

## 未实施与 3B-2 建议

本轮没有开发自动续期、多账号、正式版集成或长期常驻服务。没有重新验证 Google 登录页面（3A 已验收），没有进行失效账号、删除硬件密钥、设备重启、断电、长时后台存活等破坏性或扩展测试。

建议下一阶段先经用户确认范围：明确持久服务与探针状态的区别；补充取消/系统回收及启动中断处理；验证过期状态的明确错误与用户主动重新登录流程；为 403 保留固定枚举诊断而不是自动导入浏览器 Cookie；评估父目录同步及更新中断恢复。自动续期和多账号应另行授权，不由本轮自动展开。

不合并稳定分支，验收完成即停止。
