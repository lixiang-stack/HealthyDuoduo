# Android 本地打包与真机体验

面向 `apps/mobile` 的本地 release APK 构建，以及用 Android 真机 + Expo Go 的 USB 调试流程。云端构建见文末「备选：EAS 云构建」。

> 只想快速体验产品：见下文「真机体验（USB）」，手机连上 Mac 后即可加载当前代码。

实测版本：Expo SDK 57 / React Native 0.86.3 / Gradle 9.3.1 / AGP 8.12 / JDK 21（SDK 58 升级后尚未重新跑通本地 APK 构建）。

## 前置条件

| 依赖 | 版本 / 要求 | 说明 |
| --- | --- | --- |
| Node.js | 与仓库开发环境一致 | 在 `apps/mobile` 已执行 `npm ci` |
| JDK | **21**（本流程实测） | 用 JDK 25 会在 CMake 配置阶段报 `A restricted method in java.lang.System has been called` |
| Android SDK | platform 36、build-tools 36.0.0 | `ANDROID_HOME` 指向 SDK 根目录 |
| CMake | 3.22.1（自带 ninja） | `expo-modules-core` 等含 C++；缺 ninja 会报 `Could not find Ninja on PATH or in SDK CMake bin folders` |
| NDK | 27.1.12297006 | Gradle 会按 `ndkVersion` 自动下载；也可用 `sdkmanager` 手动安装 |

macOS 安装示例：

```bash
# JDK 21（免 sudo，装到用户目录）
curl -fL "https://api.adoptium.net/v3/binary/latest/21/ga/mac/aarch64/jdk/hotspot/normal/eclipse" -o /tmp/temurin21.tar.gz
mkdir -p ~/Library/Java/JavaVirtualMachines/temurin-21.jdk
tar -xzf /tmp/temurin21.tar.gz -C ~/Library/Java/JavaVirtualMachines/temurin-21.jdk --strip-components=1

# sdkmanager（未装 cmdline-tools 时）
brew install --cask android-commandlinetools

# CMake（内含 ninja）；NDK 可选，Gradle 会自动补
SDKMANAGER=/opt/homebrew/share/android-commandlinetools/cmdline-tools/latest/bin/sdkmanager
yes | "$SDKMANAGER" --sdk_root="$HOME/Library/Android/sdk" "cmake;3.22.1"
```

## 仓库内已固定的配置

- `app.json` → `android.package = com.healthyduoduo.app`：APK 唯一标识，缺失时 `expo prebuild` 会交互式索要。
- `app.json` → `expo-build-properties` 的 `android.usesCleartextTraffic = true`：后端是局域网明文 HTTP（如 `http://192.168.x.x:8080/api/v1`），Android 9+ 默认拦截。后端切到 HTTPS 后应删除该开关。
- `android/` 由 CNG 生成且已 gitignore，**不要手改**；原生行为一律通过 `app.json` / 配置插件调整。
- `dist/` 已 gitignore，仅存放本地产物。

## 构建

```bash
cd apps/mobile
export ANDROID_HOME="$HOME/Library/Android/sdk"
export ANDROID_SDK_ROOT="$ANDROID_HOME"
export JAVA_HOME="$HOME/Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home"
export NODE_ENV=production

npx expo prebuild -p android --clean
cd android
./gradlew assembleRelease --no-daemon -PreactNativeArchitectures=arm64-v8a
```

产物：

```
apps/mobile/android/app/build/outputs/apk/release/app-release.apk
```

复制到忽略目录便于分发：

```bash
cd apps/mobile
mkdir -p dist
cp android/app/build/outputs/apk/release/app-release.apk \
   dist/healthyduoduo-1.0.0-arm64-v8a.apk
```

要点：

- `-PreactNativeArchitectures` 决定 ABI。只给真机装用 `arm64-v8a`（体积最小，覆盖绝大多数手机）；需要其他 ABI 时用逗号分隔追加。
- `--clean` 会重新生成 `android/`，并把 `package.json` 的 `android` / `ios` 脚本改成 `expo run:*`；不希望改动的话，构建后还原这两行。
- 首次构建会下载 Gradle 9.3.1 与 Maven 依赖，之后走缓存。

## 校验

```bash
cd apps/mobile
BT="$HOME/Library/Android/sdk/build-tools/36.0.0"
"$BT/aapt2" dump badging dist/healthyduoduo-1.0.0-arm64-v8a.apk \
  | grep -E "^package|minSdkVersion|targetSdkVersion|native-code"
"$BT/apksigner" verify --print-certs dist/healthyduoduo-1.0.0-arm64-v8a.apk
```

预期：`package: name='com.healthyduoduo.app'`、`minSdkVersion:'24'`、`targetSdkVersion:'36'`、`native-code: 'arm64-v8a'`。

## 安装

```bash
~/Library/Android/sdk/platform-tools/adb install -r apps/mobile/dist/healthyduoduo-1.0.0-arm64-v8a.apk
```

也可以把 APK 传到手机直接安装。release 包没有 Metro，首次打开需到「设置」填后端地址（`http://<电脑局域网 IP>:8080/api/v1`）并点健康检查。

## 隔离真机验收（涉及迁移 / 成员模型时必读）

共享 `deploy` 栈的数据库可能含验收数据；成员/报告迁移可能是破坏性的（例如清空成员、解除报告归属）。**不要把 App 直接指向共享 `8080` 做这类验收。**

用隔离栈脚本启动一次性 Postgres、worktree 识别服务、worktree hdd 与 worktree Metro；对象存储复用本机 `:8333` 的 SeaweedFS，但使用独立 bucket：

```bash
bash scripts/android-acceptance.sh --seed
```

`--seed` 会创建一名成员、两条身高体重记录、两份肝功报告和一份合成病历，便于直接验收分析页；不加时数据库为空。

脚本会：

- 启动一次性 `postgres:17` 与独立数据库；
- 用当前 worktree 代码启动 recognizer / hdd / Metro（默认 18000 / 18080 / 18081）；
- 建立 `adb reverse` 并尝试打开 Expo Go；
- 打印 App 应填的 Base URL（`http://127.0.0.1:18080/api/v1`）；
- 按 `Ctrl+C` 后停止服务、删除数据库与对象桶、移除 reverse。

验收结束后，记得把 App「设置 → 后端 Base URL」恢复为原来的地址。

### 手动排查已有环境

```bash
lsof -nP -iTCP:8081 -sTCP:LISTEN   # Metro 属于哪个检出
lsof -nP -iTCP:18080 -iTCP:18081 -sTCP:LISTEN
"$HOME/Library/Android/sdk/platform-tools/adb" reverse --list
```

---

## 真机体验（USB）

目的：Android 手机通过数据线连 Mac，用 Expo Go 加载当前代码，直接体验 App。全程走 USB，不依赖 Wi-Fi。

### 1. 开启 USB 调试

手机上：

1. 设置 → 关于手机 → 连续点「版本号」7 次，开启开发者选项。
2. 开发者选项 → 打开「USB 调试」。
3. 插线后，通知栏把 USB 用途改为「文件传输 / MTP」。
4. 弹出「允许 USB 调试吗？」时允许（可勾选始终允许）。
5. 小米 / OPPO / vivo 等还需打开「USB 调试（安全设置）」「USB 安装」等开关。

Mac 上确认设备：

```bash
ADB="$HOME/Library/Android/sdk/platform-tools/adb"
"$ADB" kill-server
"$ADB" start-server
"$ADB" devices -l
```

必须看到 `device`：

```text
<序列号>  device  product:... model:...
```

`unauthorized`：解锁手机，点允许 USB 调试。空列表：换数据线 / USB 口，确认不是「仅充电」，再重试。`system_profiler SPUSBDataType` 里看不到手机时，问题在 USB 线 / 口 / 手机 USB 模式，不在 Expo。

### 2. USB 端口映射

把手机的 `localhost:8081`（Metro）和 `localhost:8080`（数据服务）转发到 Mac：

```bash
"$ADB" reverse tcp:8081 tcp:8081
"$ADB" reverse tcp:8080 tcp:8080
```

### 3. 启动 Metro 并打开 Expo Go

```bash
cd apps/mobile
npx expo start --android --localhost
```

- `--localhost` 使用 `exp://127.0.0.1:8081`，通过 USB 映射访问 Mac，不依赖 Wi-Fi。
- Expo CLI 会在已连接的真机上打开 Expo Go；没装会自动安装匹配 SDK 58 的版本，也可从应用商店安装最新版 Expo Go。
- 如果同时连接了多台 Android 设备，Expo 可能选错；先断开其他设备，或在终端按 `a` 后选择手机。

若没有自动打开，另开一个终端执行：

```bash
"$ADB" shell am start -a android.intent.action.VIEW -d "exp://127.0.0.1:8081"
```

### 4. 配置后端地址

前提：宿主机 `:8080` 已有数据服务（共享 deploy 栈或本地 `hdd serve`）。App 首次打开会进入「设置」，Base URL 填：

```text
http://localhost:8080/api/v1
```

因为已 `adb reverse tcp:8080`，手机上的 `localhost` 会转发到 Mac 的 8080。点「保存并校验」，地址会持久化。

### 5. 鼠标控制（可选）

USB 识别正常后，用 scrcpy 把手机画面投到 Mac，鼠标直接操作：

```bash
brew install scrcpy
scrcpy
```

### 6. 手工点检

需要截图或排查可点击区域时：

```bash
"$ADB" exec-out screencap -p > /tmp/android.png
"$ADB" shell uiautomator dump /sdcard/window.xml
"$ADB" shell cat /sdcard/window.xml | tr '>' '>\n' | rg 'tab-family|tab-archive|trend-detail'
```

也可用 `scripts/android-device.sh` 减少手算坐标：

```bash
bash scripts/android-device.sh screenshot /tmp/android.png
bash scripts/android-device.sh tap-testid member-growth-action-1
bash scripts/android-device.sh tap-text 设置
bash scripts/android-device.sh reverse 8080
```

建议至少点检：

- 底部「家庭 / 档案」图标：房子 / 三横线，激活蓝色、未激活灰色；不应出现倒三角或方块。
- 「档案 → 按项目 → 血常规」：异常项趋势层的左上角返回键可点；同日多份报告只显示一次日期，后续行显示「同日」。
- 项目详情左上角返回键可回到「按项目」。
- 冷启动后成员 / 报告数据可加载（验证 `localhost:8080` 后端地址）。

### 6.1 App 改动验收清单（人工 smoke）

每次合并 App/UI 改动前，先跑 `npm test`；再按下面清单在真机点一遍（约 10–15 分钟）。这是 ADR-0013 的人工验收，不替代 RNTL 屏幕测试。

- 设置：保存 Base URL 并通过健康检查；冷启动后地址仍生效。
- 家庭页：成员卡待核对 / 异常文案、双 Tab、设置入口、上传 FAB 正常。
- 成员：建 / 改名 / 改性别与生日（性别下拉、年月日滚轮）；孩子 / 成人由生日自动计算。删除无报告成员；删除有报告成员时确认面板显示「及其 N 份报告 · 不可恢复」，删除后当前成员清空且列表移除。
- 身高体重：成员行「身高体重」展开表单，填测量日期 / 身高 / 体重（至少一项）/ 来源（实测 / 体检单）；保存后成员行显示最近一次记录、来源与日期。
- 解读：项目详情「解读这个项目」旁显示今日剩余解读次数；先弹时间范围（6 / 12 / 24，默认 12），首次再弹数据去向确认，确认后进结果页（纳入范围、偏离项、判读标签、历史比较、病历诊断与处理原文、未纳入原因、今日剩余解读次数、配额用尽提示、来源声明）；再次进入不再弹确认；单指标「解读这个指标」提示已扩展至所属项目。
- 分析判读：肝功显示 R 值 / 转氨酶分级 / AST/ALT；血常规显示形态学与 Hb 切点；有身高体重时显示 BMI 与 WHO z / 百分位；长 value 不得挤压标签标题。
- 分析反馈：结果页「这不对」→ 填理由 → 提交后显示「已记录」；服务端 `analysis_feedback` 有对应行。
- 配额：当日解释层调用用满 20 次后重新加载结果页，显示「今日额度已用完」与「今日剩余解读次数 0 次」，反馈按钮仍可点。
- 上传：单图、多图、重复图、failed 报告原因；相机 / 相册权限弹窗正常。
- 时间线：新报告出现；待核对 / 识别失败 / 按项目筛选结果正确。
- 报告详情：指标项分组、影像段落、原图展开 / 收起、failed 形态。
- 原图查看器：点原图打开全屏；双指缩放 1×–4×、单指拖动、双击 1×/2.5×、重置；关闭后回原页面。
- 核对：确认、修正、可补录未知、未检测；徽标与剩余项随之更新。
- 趋势：异常分组、点行开详情、点选、原图定位、返回保留选中点。
- 按项目：项目列表 / 详情 / `?panel=` 筛选、最新失败报告提示。
- 成员交互态：编辑 / 身高体重 / 删除确认时，左上角返回先取消当前操作，不离开成员页；添加成员卡片隐藏；Android 硬件返回同样取消。
- 解读入口时机：项目详情不常驻时间范围；点击「解读这个项目 / 指标 / 病历」后才弹时间范围；首次再弹数据去向确认，之后不再弹；单指标提示已扩展至所属项目。
- 命名与来源：行内操作名「身高体重」；来源只有实测 / 体检单，手输不再出现。
- 原生交互：Android 返回关闭 Modal / 返回上一屏；键盘不遮输入；safe area 正常。
- 视觉：中文文案层级、颜色强调、长名称 / 空态 / 错误态；需要时用 `screencap` 留截图。

### 7. 收尾

```bash
"$ADB" reverse --remove-all
```

Metro 用 `Ctrl+C` 停止，然后拔线。

## 版本与签名

- release 当前使用模板自带的 **debug keystore** 签名，仅适合内部试用；上架 Google Play 需替换为自有 keystore（或走 EAS）。
- 发新版前自增 `app.json` 的 `version`，并建议显式设置 `android.versionCode`：未设置时默认 1，Google Play 要求每次上架的 versionCode 递增；`adb install -r` 允许同版本覆盖。

## 常见问题

| 现象 | 原因 / 处理 |
| --- | --- |
| `Could not find Ninja on PATH or in SDK CMake bin folders` | 未装 CMake，执行上面的 `cmake;3.22.1` |
| `A restricted method in java.lang.System has been called` | JDK 版本过高，切到 JDK 21 |
| `CLEARTEXT communication to ... not permitted by network security policy` | `usesCleartextTraffic` 未生效；确认 `app.json` 配置后重新 `prebuild --clean` 并打包 |
| `[CXX5304] ... SDK XML version 4` | cmdline-tools 与 AGP 版本差异，仅告警，可忽略 |
| `userInterfaceStyle: Install expo-system-ui` | 未装 `expo-system-ui`，主题固定 light，仅提示 |

## 备选：EAS 云构建

需要 Expo 账号并登录：

```bash
cd apps/mobile
npx eas-cli@latest login
npx eas-cli@latest build -p android --profile preview
```

仓库当前没有 `eas.json`，首次使用需先 `npx eas-cli@latest build:configure`，并在对应 profile 中设置 `android.buildType: "apk"`（默认产物为 AAB）。云构建消耗 EAS 配额。
