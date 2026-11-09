# Android GeckoView / PG 手动认证更新实验

当前分支 `feature/android-geckoview-auth-3b2` 从已验收 3B-1 源码 d22c5d6 创建。手动同一账号更新、校验后取消、新加密状态恢复、真实生成与冷启动已实机验证。详情、离线分类测试、未测边界及构建记录见 [3B-2 报告](AUTH_3B2_REPORT.md)。没有自然失效账号，不宣称真实过期恢复通过。

## 操作

- **检查已保存账号**：仅从原 Keystore 和固定私有目录加载，不读取浏览器 Cookie。
- **重新登录 Google**：打开原 GeckoView，保留 profile，用户自行完成登录；程序不填表或提交密码。
- **手动更新 PG 认证**：进入浏览器页面，用户确认登录完成后才读取候选 Cookie。账号、模型和 WAA 校验通过，再由用户确认替换。
- **取消认证更新**：提交确认之前取消，保留旧状态；确认替换后的短暂原子提交区不可取消，结果未知时重新检查账号。

同一账号限制由 PG 验证后的身份决定，其他账号不能替换。新状态在私有目录通过原 AES-GCM codec 加密、重读验证，再原子发布；上一份完整密文保留为 storage-state.json.previous。恢复只读固定目录，不扫描旧 3A run 目录，也不自动发布未提交候选。

首次成功检查后启用本版冷启动 Ready 检查；不自动生成或读取浏览器 Cookie。探针完成后关闭 Go runtime，Ready 不是常驻服务承诺。403 不直接判定为登录失效；只有明确失效证据才提示 AUTH_REQUIRED，其他未知原因保持 UNKNOWN_ERROR。

## 构建与升级

按 android/native/README.md 准备固定 NDK r30 / SpiderMonkey 和前端资源。Linux/WSL 设置 ANDROID_NDK_HOME、SPIDERMONKEY_ROOT 后：

```sh
bash experiments/geckoview-auth/build-auth-probe.sh
go test -race -count=1 -timeout=60s ./internal/authpoc
```

Windows 设置 JDK 21 JAVA_HOME、Android SDK local.properties 后：

```powershell
.\android\gradlew.bat -p experiments/geckoview-auth clean assembleDebug lintDebug
adb install -r experiments/geckoview-auth/app/build/outputs/apk/debug/app-debug.apk
```

包名固定 io.github.aistudio2api.authpoc；必须核对原签名，不卸载、不清数据、不更换 Keystore。稳定 APK 与实验目录独立。不得把 auth 明文、Cookie、OAuth token 或 profile 加入 Git/日志/公共存储。

本轮不开发后台自动续期、多账号、常驻服务或稳定版合并。第二阶段历史证据保留在 COOKIE_METADATA_REPORT.md；第一阶段 TEST_REPORT.md 的旧网络阻塞不是当前结论。
