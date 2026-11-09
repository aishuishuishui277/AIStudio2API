# Android 3C-1 独立集成实验

## 固定来源与隔离

- 基线：`58095c55e80aa2d415d172fe9c416a22feb1285d`。
- 选择性认证来源：`af64f32ccd43eddd53624fcc7ad5ceac8e8a77ff`。
- 分支：`feature/android-full-integration-3c1`。
- 包名：`io.github.aistudio2api.integration3c1`，ARM64，Android 15+。
- API：`http://127.0.0.1:2148/v1`；预览监听 2149。
- 不读取其他包的账号、Keystore、profile 或运行数据；不修改 VPN。

## 使用

1. 在现有 VPN 的应用分流中覆盖新包（如需要，由用户配置）。
2. 点击 Google 登录，由用户亲自完成登录。
3. 用户在浏览器页面点击手动更新 PG 认证，确认后读取 Cookie。
4. 候选在私有暂存目录加密、读取验证，验证账号、模型和 WAA 后再次确认保存。
5. 首次保存登记唯一账号；以后只能更新同一账号，保留 previous 加密备份。
6. 返回主界面启动本地服务。服务持续监听直到手动停止，通知也可停止。
7. 复制 API 地址和 API Key 到客户端。管理页用户名 admin，密码由应用随机初始化，可在主界面复制。
8. 更新认证或独立检查账号前，先停止本应用 2148 服务。跨进程文件锁阻止并发修改。

默认不自动启动；用户勾选后，打开应用时恢复已保存账号并启动服务。
冷启动不读取浏览器 Cookie，不自动生成文本。检查账号也不初始化 GeckoView。
截图未受限制。调试包不含后台自动续期、多账号管理或开机自启。

## 构建

保持与既有 Android 基线相同的 NDK r30 和已构建 SpiderMonkey 包。

```sh
npm --prefix web ci --no-audit --no-fund
export ANDROID_NDK_HOME=/path/to/android-ndk-r30
export SPIDERMONKEY_ROOT=/path/to/android-spidermonkey-package
bash android/native/build-core.sh
```

配置 Android SDK（local.properties，不提交），使用 JDK 21：

```powershell
cd android
.\gradlew.bat :app:assembleDebug --offline --console=plain
```

原生构建同时产生服务 libaistudio2api.so 和独立认证进程 libauthprobe.so。
Gradle 使用已固定版本的 GeckoView；首次联网下载依赖时去掉 --offline。
安装仅针对新包；禁止卸载或清理两个既有包。

## 数据与日志边界

固定账号目录 files/core/auth；候选 files/auth-candidates。两者均为应用私有目录。
同一新应用 Keystore 包装随机状态密钥；沿用 PG Android AES-GCM codec。
首次发布整个已验证账号树；后续沿用已验证的密文原子替换和 previous 备份。
操作取消、暂存残留不会在启动时自动发布；不扫描旧 3A run 目录。
真实 Cookie 值仅在认证通道内存及加密状态中存在，不进入 UI、日志或工件。
集成构建关闭请求正文捕获和原始 PG 日志，原生界面仅显示脱敏状态。
模型映射与按请求 fake stream 保留，仅为实际存在的上游模型展示别名。

## 最终验收（2026-11-09）

设备 vivo V2444A / Android 15 / ARM64。按授权完成两次直接生成，无自动重试。

| 项目 | 结果 | 证据与边界 |
| --- | --- | --- |
| 离线状态机、首次发布、覆盖保护 | PASS | race 测试；不代替真实认证 |
| Cookie hostOnly/SameSite、模型映射、fake stream 回归 | PASS | 定向源码测试 |
| Kotlin、WebUI、双 ARM64 原生库、debug APK | PASS | Gradle 与 native 构建日志；16 KB LOAD 对齐 |
| 新包安装、原生界面、ADB 截图 | PASS | 新包 UID 10490 |
| 与 stable / authpoc 并存 | PASS | UID 10486 / 10489；原包版本、更新时间未变 |
| 新包 Google 登录 | PASS（用户实测） | 用户亲自登录并确认进入 AI Studio |
| 首次认证提交 | PASS | committed、directory_synced、encrypted_state_reload 均 true |
| 浏览器 Cookie 读取 | 1 次 | 用户主动导入；服务启动、两次生成及重启后累计仍为 1 |
| 持续服务 /health | PASS | 127.0.0.1:2148，独立 HTTP 200，最终复核仍 200 |
| /v1/models | PASS | HTTP 200，97 个对外名称，含派生及假流别名；不能称为 97 个独立上游模型 |
| gemini-3.8-flash 非流式 | PASS | HTTP 200、合法 JSON/choices、正文 OK、finish_reason=stop，4227 ms |
| gemini-3.8-flash SSE | PASS | HTTP 200、2 个非空内容 delta、131 字符、stop 与 [DONE]，2385 ms，无错误事件 |
| 真流配置 | PASS | 全局 FAKE_STREAM 默认 false；真实模型不在 fake 列表，假流别名在列表 |
| 用户手动停止 | PASS | 手机本机 nc 对 2148、2149 均返回 Connection refused |
| 服务重新启动 | PASS | 健康和模型目录再次 HTTP 200，真实/假流名称存在，新增 Cookie 读取 0 |
| 手机 SillyTavern 两轮 | PASS（用户实测和截图） | 127.0.0.1:8000 聊天页；用户确认连接 2148、gemini-3.8-flash、逐步输出、无重复 Assistant 消息 |
| 安装包与交付包一致 | PASS | 手机 base.apk 与交付 APK SHA256 完全一致 |

SSE 两个内容事件在客户端记录为同一毫秒到达；通过的是真实流分支、多内容事件与完整协议结束，不能据此承诺平滑逐字显示。
SillyTavern 首次失败为仍连接旧 2048 的 ECONNREFUSED，改用 2148 后成功；不是 Google 认证失效。
两轮已满足本阶段“最多 3 轮”范围，不额外生成。无实机失败驱动的源码修复，修复轮数 0/3。

## 未运行项目及限制

- 本新包未再次执行整个 APK force-stop 冷启动验收；本轮已实测服务停止、重新启动和已保存账号恢复，不沿用旧 PoC 的结果冒充新包 PASS。
- 未重新实测新包内的同账号二次替换、取消、previous 回滚或自然过期；复用 3B-2，离线状态机已通过。
- 假流别名与定向回归通过，但本轮未额外向 Google 发起假流生成。
- 不含后台自动续期、多账号管理；未做 30 分钟压力测试。
- Android 系统仍可能回收进程；无无限重启保证。启动偏好默认关闭。
- 为防敏感数据进入日志，集成版关闭原始 PG 请求日志和请求正文捕获，仅保留脱敏状态、生命周期日志和计数。
- 本机 ss 的 netlink 检查权限不足；停止验收改用手机本机 nc，明确收到连接拒绝。

## 交付与回退

保留已安装 3C-1 服务运行，未再读取 Cookie 或生成。原 stable 与 authpoc 未停止、卸载、更新或清数据。
如需回到旧稳定版，先在 3C-1 点击停止，客户端改回原稳定版地址及对应 Key；自行启动旧稳定版。不要删除新包数据或跨包复制账号。
本阶段只提交独立实验分支，不合并原分支，不构建 release。源码提交和报告提交分开；APK 源码提交见下方。

## 初次安装产物

源码提交：`37250acaa77f79307dafcaabc94225062823fd11`。

APK SHA256：`d834fa45c79b630ab441e45c14a6293ac0bc0462dde6127a647fd2238447b700`。

签名证书 SHA256：`01de4840d8ff72e3f7d150ec57ca2d5288e1cdf1e28efd7ed870133f00d0c783`。

以上 APK 已完成本轮验收，未因文档更新重编译或重新安装。
