# Android 本地打包与模拟器验证

面向 `apps/mobile` 的本地 release APK 构建，以及用 Android 模拟器 + Expo Go 跑真实 App 的手工验证流程。云端构建见文末「备选：EAS 云构建」。

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

- `-PreactNativeArchitectures` 决定 ABI。只给真机装用 `arm64-v8a`（体积最小，覆盖绝大多数手机）；需要模拟器再追加 `x86_64`（逗号分隔）。
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

## 模拟器验证（Expo Go）

用于在无 iOS 模拟器 / 无真机时，快速验证 Android 原生渲染与交互。它不是 E2E；Web 自动化仍跑 `npm run e2e`。

以下命令用两个变量简化路径：

```bash
ADB="$HOME/Library/Android/sdk/platform-tools/adb"
EMULATOR="$HOME/Library/Android/sdk/emulator/emulator"
```

### 1. 启动模拟器

```bash
"$EMULATOR" -list-avds
"$EMULATOR" -avd <AVD_NAME> \
  -no-snapshot -no-audio -no-boot-anim -gpu swiftshader_indirect
```

本机实测使用 `Medium_Phone_API_37.0`（1080×2400，Android 16 / API 37）。确认启动完成：

```bash
"$ADB" devices
"$ADB" shell getprop sys.boot_completed
# 输出 1
```

### 2. 启动 Metro

```bash
cd apps/mobile
npx expo start
```

保持终端运行。默认端口 `8081`；若被占用，用 `npx expo start --port 8082`，并同步修改下面的 `adb reverse` 与 `exp://` 端口。

### 3. 安装 Expo Go（SDK 58）

模拟器首次使用需安装与项目 SDK 匹配的 Expo Go。APK 版本以 [expo-go-releases](https://github.com/expo/expo-go-releases) 为准；当前 SDK 58 可用：

```bash
curl -L --fail -o /tmp/expo-go-58.apk \
  https://github.com/expo/expo-go-releases/releases/download/Expo-Go-58.0.5/Expo-Go-58.0.5.apk
"$ADB" install -r /tmp/expo-go-58.apk
```

### 4. 端口映射并打开项目

模拟器里的 `localhost` 指向模拟器自身；把 Metro / 数据服务映射到宿主机：

```bash
"$ADB" reverse tcp:8081 tcp:8081
"$ADB" reverse tcp:8080 tcp:8080
"$ADB" shell am start \
  -a android.intent.action.VIEW -d "exp://127.0.0.1:8081"
```

首次打开会弹 Expo Go 开发者菜单，点「Continue」。

### 5. 配置后端地址

首次安装后 App 没有本地存储的地址。进入「设置」填：

```text
http://localhost:8080/api/v1
```

（已做 `adb reverse tcp:8080` 时可用 `localhost`；也可填电脑局域网地址，如 `http://192.168.x.x:8080/api/v1`。）

### 6. 手工点检与操作

截图：

```bash
"$ADB" exec-out screencap -p > /tmp/android.png
```

点按 / 滑动：

```bash
# 坐标为 1080×2400 模拟器；换设备前先用 uiautomator 查 bounds
"$ADB" shell input tap 540 1200
"$ADB" shell input swipe 540 1800 540 800 300
```

查 testID / 可点击区域（RNW 的 testID 会出现在 `resource-id`）：

```bash
"$ADB" shell uiautomator dump /sdcard/window.xml
"$ADB" shell cat /sdcard/window.xml | tr '>' '>\n' | rg 'tab-family|tab-archive|trend-detail'
```

建议至少点检：

- 底部「家庭 / 档案」图标：房子 / 三横线，激活蓝色、未激活灰色；不应出现倒三角或方块。
- 「档案 → 按项目 → 血常规」：异常项趋势层的左上角返回键可点；同日多份报告只显示一次日期，后续行显示「同日」。
- 项目详情左上角返回键可回到「按项目」。
- 冷启动后成员 / 报告数据可加载（验证 `adb reverse` 与后端地址）。

### 7. 收尾

```bash
"$ADB" emu kill
```

Metro 用 `Ctrl+C` 停止。

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
