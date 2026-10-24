# Android APK 本地打包

面向 `apps/mobile` 的本地 release APK 构建流程。云端构建见文末「备选：EAS 云构建」。

实测版本：Expo SDK 57 / React Native 0.86.3 / Gradle 9.3.1 / AGP 8.12 / JDK 21。

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
