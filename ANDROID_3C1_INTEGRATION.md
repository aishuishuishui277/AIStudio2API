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

## 验收状态（进行中）

- 离线认证状态机、原子发布测试：通过；不能代表真实认证通过。
- HostOnly / SameSite / Cookie / 模型映射 / fake stream 定向回归：通过。
- Kotlin 编译：通过。
- 完整 ARM64 APK：编译通过；核心及认证库均为 AArch64、16 KB LOAD 对齐。
- 独立安装、真实登录、认证保存、持续 API、生成与 SSE、用户停止重启、手机 SillyTavern：待验证。
- 没有执行 30 分钟压力测试；不将未运行项目记录为 PASS。

真实 Google 登录、Cookie 读取及生成须分别等待用户确认。
