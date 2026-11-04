# Android 双模式与截图验证报告

日期：2026-11-04（Asia/Shanghai）。设备：vivo V2444A / Android 15 / ARM64。交付为 debug，未构建 release。

## 版本与范围

- 项目为已核实的既有 Fork `aishuishuishui277/AIStudio2API`，origin 未改，无重新 Fork 或上游写入。
- 成功版本基线：`ff9b928ebc2fb062b41fdd5697cdaa4358fa172d`。只在 `feature/android-standalone` 开发，main 保持 `a7afc31252dbb2543d79ac664eabf569148303bc`。
- 模式功能提交：`96fa819`；APK 构建代码提交：`d4100eb16b7a29205c43a5bb23150d4cf49f6624`，Go buildinfo 的 vcs.modified=false。后续文档提交不改变 APK 代码。
- 包名 `io.github.aistudio2api.android`，版本 `0.2.0-android-dual-stream`，versionCode 2；沿用 debug 签名 SHA256 `01de4840d8ff72e3f7d150ec57ca2d5288e1cdf1e28efd7ed870133f00d0c783`。
- APK SHA256：`c9468f410c6d70d056d077c523ad0361a1b94691e13b5ee190b83185acbbc8b5`，91,823,203 字节。从设备取回 base.apk 摘要一致。
- 不修改 Google 认证、SpiderMonkey 或 Playground 协议。升级前保留成功 APK；仅 install -r 升级（最终使用 --no-streaming 安装传输选项），无卸载、清数据或权限绕过。

## 实现

`FAKE_STREAM_MODELS` 是逗号分隔的原始客户端模型名。API 在映射前决定每次请求的 Unary，再复用原模型映射和已有 FAKE_STREAM / SSE；stream=false 保持 JSON，全局 FAKE_STREAM=true 优先。没有单次请求修改全局配置，没有重写 SSE。

公开模型投影复用 MODEL_MAPPING，复制真实目标的能力与权限数据。配置为假流的别名要求精确真实目录 ID，不能利用 Google capability aliases 把旧型号回退为 latest 并宣称旧型号存在；普通既有映射行为保留。虚拟 ID 不进入原目录缓存。Android 一次性迁移仅补两个配置项，保留原文件备份、既有映射冲突、账号和其他设置。

Android 普通管理 WebView 移除唯一 FLAG_SECURE 调用及对应 import；没有移除 Keystore/AES-GCM/私有存储/API Key/脱敏。测试中只通过既有服务发起请求，未提取 Google Cookie 或密码。

## 自动化与实机结果

| 场景 | 结果 | 证据范围 |
| --- | --- | --- |
| 配置读写、环境覆盖、迁移备份/幂等/退出默认值 | PASS | Go config 测试，验证代理/API key 字符串及未知配置行原样保留 |
| API 模式选择 / stream=false / 全局开关 | PASS | vivo 离线 API 测试 30 个顶层测试；包含 16 种主力型号/别名/流式组合 |
| 并发状态竞争 | PASS（测试范围内） | Linux amd64 config/api/app 完整 go test -race；64 个混合请求无模式串扰；真机另做真实 A/B 并发 |
| 映射、能力继承、缺席型号与目录不变 | PASS | vivo app 完整 4 个顶层测试；覆盖 latest 的旧名称同义项不能虚构假流型号；真实 /api/models 无 -fake ID；/v1/models 单模型字段除 ID 外一致 |
| 真流 A：gemini-3.8-flash | PASS 实机 | HTTP200、SSE、中文三行逐字保持，role/finish/[DONE] 各一次 |
| 假流 B：gemini-3.8-flash-fake | PASS 实机 | HTTP200，复用 Unary；完整中文结果转现有 SSE，无重复/错序/换行丢失 |
| A+B 同时请求 | PASS 实机 | 两个并发请求均200，各自保留响应模型名和模式；未切全局开关 |
| C：fake + stream=false | PASS 实机 | 普通 JSON HTTP200，完整三行内容和客户端别名保留 |
| 3.1 Flash Lite 真/假 | PASS 实机 | gemini-3.1-flash-lite 和 -fake 均HTTP200、SSE完整 |
| 3.1 Pro Preview 假流 | PASS 实机 | gemini-3.1-pro-preview-fake HTTP200、SSE完整 |
| Fake 客户端取消 | PASS 实机 | 在响应头前断开，观察到活动请求后回落0；单元测试另验证 ctx 取消及 handler 返回 |
| 错误不能假装成功 | PASS（已测范围） | 注入错误返回错误而非成功/DONE；SillyTavern 的真实上游403被明确报告，没有新增空的成功角色回复 |
| 服务停止 / 重启 / 再生成 | PASS 实机 | STOPPED，约5.461秒内完成关闭，保留转发时监听关闭；重启health200，重启后fake非流式生成200。JVM/Go进程可以暂留，不以 PID 消失替代资源验收 |
| APK升级与身份 | PASS 实机 | 最终 install -r 成功；设备 APK 与交付摘要相同；签名、应用ID不变 |
| 私有配置与 Keystore 保留 | PASS 实机 | PROXY/API key 行摘要、vault包装文件摘要及账号数量升级前后保持，已有加密账号可继续生成 |
| Android管理页 ADB截图 | PASS 实机 | WebView完整模型页面可见，不是黑块 |
| Android系统截图 | PASS 实机 | KEYCODE_SYSRQ 产生新的系统 Screenshot JPEG，已取回 |
| Android录屏 | PASS 实机 | screenrecord录制5秒，解码视频实际帧看到完整WebView内容 |
| 日志页面截图 | PASS 实机 | APK默认管理入口的日志页内容可见，日志中实际目标为gemini-3.8-flash / Playground |
| 构建、Lint、16KB静态检查 | PASS | Go/NDK核心、WebUI typecheck/build、Gradle clean assembleDebug/lintDebug、签名、ZIP/ELF对齐；有既有Lint/构建警告 |

真实短文本测试的真流与假流都可能只有少量 chunk；本次3.8真流2个内容chunk，假流2个紧邻chunk（约1ms内输出），3.1 Pro假流2个chunk。chunk数量不等于流式模式；选择依据为原始名称对应的请求 Unary，沿用既有完整结果路径。

## SillyTavern 验收

用户授权使用隔离客户端。使用官方 SillyTavern 1.19.0，固定提交 `7e8663cd9c184a550b37238218bdd32c6efc68e9`，独立 Windows 数据目录。浏览器实际操作，客户端通过临时 ADB loopback 转发访问 vivo APK 的2048端口；不是 Windows AIStudio2API 后端。手机实际使用地址仍为 `http://127.0.0.1:2048/v1`。

加载独立合成角色卡“青灯掌柜-双模型验收”，同一会话先A三轮，再切B三轮。模型列表真实读取，A/B均可选。UI与保存的chat记录确认：6条用户消息、6条生成回复（另有1条角色开场），两款模型各3条回复，内容无重复，最后正确记住“林舟”和“渡口”。保留了角色、上下文、中文及换行。

**发生过1次上游HTTP403**：真流第2轮 Google 返回“The caller does not have permission”，客户端报告错误；普通重试一次后成功，并继续完成其余轮次。没有改认证/WAA或隐藏这次失败，因此这不是“全程零失败”的稳定性结论。

测试结束后删除隔离客户端的临时 API key 副本，确认客户端日志未出现 key，停止临时服务并移除仅本次创建的转发。用户原有客户端、角色卡和连接配置未改。手机本地服务恢复运行。

## 当前目录与未测边界

- 3.8 Flash、3.1 Pro Preview、3.1 Flash Lite、3 Flash Preview等实时存在型号及其fake别名已验证目录。2.5 Pro、3 Pro Preview、普通3.1 Flash目前没有独立真实目录项，不能凭配置宣称可用；这些旧型号已预置映射，实际生成标 **Not Run / 当前目录缺席**。
- 其他可见主力别名仅做目录/通用映射验证，没有逐个真实生成验收。用单元测试中的合成模型不代表该Google型号可用。
- 用户原手机 SillyTavern/Tavo 客户端本轮 **Not Run**；本轮六轮数据来自授权的隔离 SillyTavern 浏览器客户端。
- 本轮不包含30分钟负载、锁屏、网络切换、第三方录屏App、16KB页实机或完整Windows发布构建；均 **Not Run**，不从编译或短测推断通过。
- Google偶发403仍可发生；本轮未修改该上游权限/风控机制。方案B直接登录仍非本轮范围。

## 构建、配置与回滚

操作步骤见 `android/README.md`、`android/native/README.md`。新配置键和格式见 `.env.example`。升级先停止本地服务，再 `adb install -r`；某些vivo安装等待时可在停服后使用 `adb install --no-streaming -r`，该选项只改变ADB安装传输。

交付目录保留旧成功APK（SHA256 `4ffc90a63dfb851a06a171f558c6cad8535f498ac8161da78ec62e338e75acb4`）。需要回滚时停服后 `adb install -r -d <旧debug.apk>`，必要时恢复应用私有 `.env.before-model-modes-v1`，不卸载、不清账号或Keystore。恢复原配置前先保留升级后的个人修改。

交付证据包含公开模型投影、真实SSE、RP摘要/截图、系统截图/录屏、启停、APK身份及测试日志；不包含账号JSON、Cookie、API key或Keystore材料。文件摘要与文档提交另记于DELIVERY.json。
