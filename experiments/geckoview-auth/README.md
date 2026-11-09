# Android GeckoView / PG 认证实验

当前独立分支：`feature/android-geckoview-auth-3b1`。3B-1 已完成本机真实验收：加密账号持久化、Keystore 恢复、PG 进程重启与整包冷启动恢复，最终真实生成 HTTP 200 / OK。首次生成出现过一次 403，具体原因未确定；详见 [3B-1 完整报告](AUTH_3B1_REPORT.md)。没有进入 3B-2、自动续期、多账号或稳定版集成。

## 使用

实验包 ID 保持 `io.github.aistudio2api.authpoc`，必须核对原签名后用 `adb install -r` 保留数据升级。不要卸载或清数据；稳定 APK 独立保留。

默认页为纯原生恢复页，不启动 GeckoView：

- **启动已保存的 PG 账号**：首次从既有 3A 加密原件迁移到固定私有目录，后续仅加载固定目录并检查 Ready。
- **验证已保存账号生成（一次）**：使用固定目录发起指定短文本请求，不读取浏览器 Cookie。
- **打开原 GeckoView 登录 PoC**：保留原浏览器、元数据检查与 3A 手动导入实验；本轮恢复测试没有进入该页面。

首次 Ready 成功后，冷启动自动恢复并检查 Ready；不会自动生成。每次探针结束即关闭 Go runtime，Ready 是检查结果，不是常驻 API 服务。状态无效时明确报错，不回退读取 GeckoView 或旧 run 目录。

持久目录为 `files/pg-auth-3b1/auth`，同一实验 Keystore alias 与原 AES-GCM codec。不会读取稳定 APK 私有数据或占用 2048。仅使用既有手机 VPN，不改变节点、代理或 TLS。

## 构建

先按 [3B-1 报告](AUTH_3B1_REPORT.md) 和 `android/native/README.md` 准备前端资源、NDK r30 与 SpiderMonkey，执行：

```sh
bash experiments/geckoview-auth/build-auth-probe.sh
```

设置 Android SDK / JDK 21 后，从仓库根目录执行：

```powershell
.\android\gradlew.bat -p experiments/geckoview-auth clean assembleDebug lintDebug
adb install -r experiments/geckoview-auth/app/build/outputs/apk/debug/app-debug.apk
adb shell am start -n io.github.aistudio2api.authpoc/.SavedAccountActivity
```

固定 GeckoView 159.0.20261102144711、AGP 9.4.1、Kotlin 2.4.21、Gradle 9.8.1、compileSdk 37.2、targetSdk 35；原生库必须先构建，不提交 JNI 产物、私有凭据或开发机配置。

## 历史阶段

[第二阶段报告](COOKIE_METADATA_REPORT.md) 记录用户手动登录后 23 个 Cookie 无值元数据、Native Messaging 与登录会话保留。第一阶段 `TEST_REPORT.md` 的网络阻塞是历史状态。3A 已证明 GeckoView 状态能由 PG 原认证器使用；本分支从其指定代码提交 a59288b 创建，不重新证明 Google 登录。

官方 Termux 的 Debian/Camoufox/BiDi 画面转发不同于本 Android 原生 GeckoView 方案；本机运行不需要电脑、Termux 或 VPS。实验成果尚未合并稳定分支。
