# GeckoView Cookie 元数据第二阶段报告

2026-11-09，vivo V2444A / Android 15 / ARM64。范围止于元数据与通信，不进行真实 auth 导出或导入。

## 基线与交付身份

实验工作树起点为 `a851f782e5b9d376dff26e11952b6b6e4f8eaf3a`，其中 APK 代码为用户指定的 `1e3f06e`；工作树干净。Fork API 确认 parent 为 Mag1cFall/AIStudio2API。

新版 APK 代码提交 `4a7bbc35aa117ba6501ad617483dc2d3117524ea`，包名仍为 `io.github.aistudio2api.authpoc`，versionCode 2，versionName `0.2.0-cookie-metadata-poc`。大小 209086719 bytes；SHA256 `3d85b9ee1f051a4092f1db6fe1ff7cdbdf00dbbee722a94e71c37aaae7701b89`。

构建前拉取的是已安装 APK 本身，不是私有 profile。原签名与新版签名均为 SHA256 `01de4840d8ff72e3f7d150ec57ca2d5288e1cdf1e28efd7ed870133f00d0c783`。确认包名/签名一致后执行 `adb install --no-streaming -r`，返回 Success。没有卸载、清数据或创建新 profile。

## 实测验收

| 项目 | 结果 | 证据 |
|---|---|---|
| A 编译 | PASS | assembleDebug + lintDebug；最终 BUILD SUCCESSFUL 13s，0 errors / 16 warnings |
| B 登录会话保留 | PASS（页面会话层） | 用户先前提供 Playground 实际生成截图；升级后及冷启动后仍进入 Gemini 3.8 Flash Playground，无重新输入账号；读取到原有登录 Cookie 元数据。本轮未重新发送生成请求 |
| C 内置扩展安装 | PASS | 两次启动显示“扩展已安装”；installBuiltIn 成功回调后完成真实查询 |
| D Native Messaging | PASS | connectNative 接受 Kotlin 检查命令；sendNativeMessage → MessageDelegate.onMessage 返回真实结果 |
| E Google Cookie 元数据 | PASS | 首次与冷启动后分别读取 23 个，全部通过精确字段校验 |
| F 不输出 Cookie value | PASS（本应用实现及交付件） | JS 不访问 value，发送新建白名单对象；Kotlin 再验来源、请求编号、精确键集合和类型；元数据 JSON 无 value 键。无原始 Cookie/异常日志、无控制台转发，无 profile/全量 logcat 采集 |
| G 应用重启 | PASS | 仅 force-stop PoC 后冷启动 673ms；扩展重新连接、Playground 加载；新查询文件 mtime 从 1794216098 变为 1794216172，排除读取旧结果 |

一次源码修复：Kotlin 安装结果回调的 nullable WebExtension 需显式空值处理；第二次构建及首次真实扩展测试通过，未修改 UA 或 TLS。没有使用模拟 Cookie 代替上述实测。补充的合成 getter 检查只验证投影函数不会访问 value，不作为 E 的证据。

## 实际元数据结构

查询当前扩展执行上下文的默认 Cookie store，限定 `google.com` 域族并二次筛选允许的三个精确主机。host permissions 仅 `https://google.com/*`、`https://accounts.google.com/*`、`https://aistudio.google.com/*`，没有 `<all_urls>` 或 Google 子域通配授权。结果范围不是浏览器所有 Cookie。

实际结果：23 个；domain 为 `.google.com`、`.aistudio.google.com`、`accounts.google.com`；path 全为 `/`；Secure 16 个、HttpOnly 13 个；有 expirationDate 属性 23 个；session 0；SameSite `no_restriction` 5 个、`unspecified` 18 个；hasPartitionKey 0、hasFirstPartyDomain 0。存在 SAPISID 等登录 Cookie 的名称不等于其值已被 Go 服务接受。

消息外层只有 `type,id,count,cookies`。每个条目只有 `name,domain,path,secure,httpOnly,sameSite,session,hostOnly,hasExpiration,hasPartitionKey,hasFirstPartyDomain`。未输出具体到期时间、分区键内容、Cookie value 或其 hash/长度/片段。

实际 JSON 和界面截图仅保存在本地交付目录，不提交账号相关运行数据到 Git。应用只保存一份经过校验的元数据文件，不保存原始 API 结果。WebExtension API 本身会在浏览器内存中返回 Cookie 对象；这不等于零接触的专用元数据 API。代码从对象选取属性后发送，不访问其 value，不能宣称浏览器内部完全不持有值。

## 与现有 storage-state 格式的对应

参照原 `internal/aistudio/auth.go` 的 StateCookie/StorageState 和 Validate；不读取真实 Windows 账号文件。

| Gecko 数据 | 原格式 | 判断 |
|---|---|---|
| name/domain/path | 同名字符串 | 本次结构可直接对应，保留 domain 的前导点 |
| secure/httpOnly | 同名布尔值 | 可直接对应 |
| sameSite=no_restriction/lax/strict | None/Lax/Strict | 需要显式枚举转换 |
| sameSite=unspecified | 空字符串或缺省语义 | Validate 允许空字符串，不能未经验证强制视为 Lax；实际发现 18 个 |
| expirationDate/session | expires 数值，常见会话值 -1 | 需要秒制数值/会话规则转换；本轮仅测存在性，未取得数值，未实测会话 Cookie |
| partitionKey 对象 | partitionKey 字符串 | 需研究 topLevelSite 等语义，不能直接复制；本次无分区样本，兼容性 Not Run |
| firstPartyDomain/storeId/hostOnly | 部分无直接字段 | 不能假设可无损丢弃隔离语义；本次仅默认 store |
| value | value | 本轮刻意缺失，不能生成有效 auth 文件 |
| localStorage | origins[].localStorage | 本轮没有读取，不得假设 AI Studio/Drive 所需状态齐全 |
| Chrome OAuth/设备绑定扩展 | aistudio2api 附加数据 | Gecko 元数据不提供，不能冒充 Windows Chrome 导入状态 |

因此结论仅为主要 Cookie 元数据字段可映射；不是完整 storage-state 兼容或 Go 认证 PASS。

## 官方文档核对

本轮阅读最新 Mozilla 文档，并对照当前固定 GeckoView 159 源码签名，未自动升级浏览器引擎：

- [WebExtensionController.installBuiltIn](https://mozilla.github.io/geckoview/javadoc/mozilla-central/org/mozilla/geckoview/WebExtensionController.html)：从 APK assets 安装内置扩展。
- [MessageDelegate](https://mozilla.github.io/geckoview/javadoc/mozilla-central/org/mozilla/geckoview/WebExtension.MessageDelegate.html)：消息视为不可信，onMessage 校验来源和内容。
- [cookies.getAll](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/cookies/getAll)：cookies/host 权限；`partitionKey:{}` 包含分区与非分区存储；`firstPartyDomain:null` 覆盖 first-party isolation 上下文。
- [sendNativeMessage](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/runtime/sendNativeMessage)：发送单条 JSON；GeckoView 的目标是嵌入应用 MessageDelegate，并非桌面 native-host 子进程。

扩展权限为 cookies/nativeMessaging/geckoViewAddons 加三个 hosts。无内容脚本、表单抓取、网页注入、tabs/webRequest 权限；CSP connect-src 'none'，无 fetch 或上传实现。

## 后续建议与停止边界

当前可进入另行确认后的第三阶段设计，但本轮到此停止。下一阶段应先在独立进程/测试账号路径中设计临时内存传递、最小字段转换和 Android Keystore/AES-GCM 保存，针对 SameSite 未指定、expiry、分区状态、localStorage 依赖逐项验证，再做真实 PG 认证失效/刷新/重启验证。不得直接覆盖稳定账号，不应因存在 Cookie 名称就判定 auth 可用。

没有修改正式 Go/SpiderMonkey、稳定 Android 模块、`feature/android-standalone` 或 main，没有合并实验代码。稳定包仍 versionCode 2 / 0.2.0-android-dual-stream，更新时间保持 2026-11-04 16:50:40。真实 auth 导出、导入、PG Go 生成测试均为 Not Run。
