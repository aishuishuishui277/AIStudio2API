# Android 测试报告

日期：2026-11-04。**可安装debug原型，最终业务验收未完成。** APK摘要和完整源码提交见交付目录DELIVERY.json。

## 身份与环境

Fork：aishuishuishui277/AIStudio2API，基线a7afc31252dbb2543d79ac664eabf569148303bc，分支feature/android-standalone。未重新Fork、覆盖main、强推、写上游或覆盖用户修改。

设备vivo V2444A / Android15 / arm64-v8a / 4096页。包io.github.aistudio2api.android，版本0.1.0-android-prototype/1，min/target35。Go1.27.3、NDKr30、Rust1.99.1、cbindgen0.29.4、SDK37、JDK21、Gradle9.8.1、AGP9.4.1。引擎e297ff009d3b9afee0e223c02ae3271ef24d51b6。

未读取真实账号状态或密码，未修改VPN，未清数据；功能测试时账号数为0。

## 已执行结果

| 检查 | 结果与范围 |
| --- | --- |
| Fork/Git/初始工作树 | PASS，API/gh核实后克隆、干净基线建分支 |
| npm ci/build | PASS，原锁文件与实际dist；有性能提示 |
| Go配置测试 | PASS，go test -count=1 -timeout=60s ./internal/config |
| Android依赖/完整编译 | PASS，CGO=1完整核心；CGO=0按预期失败 |
| SpiderMonkey | PASS基础运行：NDK编译/定制符号、实机双context/42求值/Intl/Promise jobs/GC关闭；不含Google proof |
| SQLite | PASS实机底层：WAL、插入、关闭重开读取约0.06秒 |
| AES-GCM | PASS合成状态：主机/实机往返、随机nonce、篡改/错误密钥/明文拒绝；无真实Cookie |
| 原API回归 | PASS设备离线：26顶层/含子测试34，0失败，不代表真实模型 |
| tokenizer/指纹 | PASS设备离线：长文本计数约0.44秒，Firefox152形状及时区生成约0.24秒 |
| Kotlin/APK/lint | PASS assembleDebug/lintDebug；保留文案/WebView JS/目标SDK等警告，无baseline掩盖错误 |
| ELF/ZIP/签名 | PASS静态：ARM64、两个库LOAD≥16KB、携带libc++_shared；zipalign16KB和签名验证。16KB设备未测 |
| APK安装/升级/JNI | PASS，vivo安装/同签名覆盖升级、JNI装载、Keystore初始化和密钥复用启动 |
| 本地API/WebUI | PASS，health200、原Web根页面200，WebView实际渲染账号/模型入口 |
| 无账号明确报错 | PASS，启动生成400 account_required；生成服务未启动时models503 service_stopped，不假装模型加载成功 |
| 启停与状态 | PASS修复后：停止UI=STOPPED，保留ADB forward时监听确实关闭；重启UI=RUNNING、health200 |
| 前台服务 | PASS运行声明：isForeground=true/type0x40000000，通知2048含停止动作；通知动作点击未测 |
| Google无账号公网诊断 | PASS代理路径：APK自身authenticated=false、HTTP302、HTTP/2.0；不是生成HTTP200 |
| 空闲内存/退出观察 | 已采样core PSS73707KB/RSS162432KB/SwapPSS5653KB；无账号、APK未启动WAA workers，不是生成峰值。观察到升级退出和WebView自退，未观察到native crash，不能代替稳定性验收 |

底层测试在设备专用/data/local/tmp/aistudio2api-android-20261104运行；APK生命周期/WebUI/Keystore/公网诊断另在应用进程验证，不混同。最后的Android账号入口提示变更单独构建检查，不将此前测试冒充该新界面的点击验收。

## 网络情况

用户手机浏览器能访问AI Studio。原Go客户端在APK直连报fake-IP 198.18.0.16:443连接拒绝，shell直连曾超时。同一客户端经**手机本地**http://127.0.0.1:7890成功HTTP302/HTTP2。仅修改本应用私有.env的PROXY，原文件留备份，未动VPN；新装默认PROXY仍为空。

当前实测需要手机本地代理可用，不需要电脑/VPS/Termux提供反代。未做拔USB后的长时验收；核心、资源、私有数据均在APK/应用内，ADB只用于安装观察，不承载运行时API或Google代理。

## 未完成验收

| 项目 | 状态 |
| --- | --- |
| Windows真实状态迁移、加密落盘/重启恢复 | Not run，需用户手机本地导入 |
| 认证过期/重新导入恢复 | Not run |
| Google完整WAA proof | Not run |
| 真实模型目录含gemini-3.8-flash | Not run，以账户ListModels为准 |
| Gemini HTTP200/文本/SSE/多轮/长上下文 | Not run |
| SillyTavern连接/模型选择/角色卡/10轮RP | Not run |
| 30分钟/锁屏/网络切换/回收/生成内存峰值 | Not run |
| 通知停止动作/端口占用实测 | Not run，代码已实现，界面停止已测 |
| 真16KB页设备/完整Windows构建 | Not run |
| Android直接Google登录 | 方案B未实现，WebView不能假定可用；GeckoView候选也未验证 |
| release APK | 未构建，等待真实验收 |

## 证据与边界

原始日志保留在工作区work：config-tests.log、native-recipe-validation.log、android-core-build.log、android-apk-build.log、device-mozjs-tests.log、device-api-tests.log、device-offline-dependency-tests.log、device-final-lifecycle.json、device-webview-check.json、device-app-network-probe.txt、device-memory-idle.log。对外交付摘录不含凭据的证据。

下一阶段需要真实Google会话，按用户要求停止自动操作。账号JSON/Cookie/密码不发聊天或GitHub，由用户在手机选择文件。没有用mock、合成Cookie、空模型列表或本地health200替代真实模型验收。
