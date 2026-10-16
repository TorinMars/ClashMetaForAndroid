## T-Agent Capture

Automatic profile updates use the interval set in each profile's properties (minimum 15 minutes; clear it to disable updates). In App Settings, enable **Pause automatic updates at night** and choose the start and end hour for a device-local quiet window. Updates due during that window run when it ends. Manual profile updates remain available. Matching start and end hours disable the window.

Source version `2.11.37.capture-scheduling` (211044) combines these settings with the HTTP/HTTPS capture feature, capture allowlists, diagnostics, and cURL script export. The `capture` build has its own launcher and application name, **T-Agent Capture**, and package ID, `com.torin.clash.capture`. It installs alongside earlier Alpha and Meta apps. Import profiles and configure a capture CA separately for the new installation.

A Graphical user interface of [Clash.Meta](https://github.com/MetaCubeX/Clash.Meta) for Android

### APK download

[Download the latest device-tested APK (ARM64, 211043)](https://github.com/TorinMars/ClashMetaForAndroid/raw/refs/heads/main/downloads/t-agent-capture-2.11.36-arm64-v8a.apk)

Version: `2.11.36.capture-scheduling`. Includes HTTP/HTTPS capture, independent capture and proxy routing scopes, cURL script export, configurable profile update intervals, and overnight quiet hours. This APK uses a separate package ID and the current local debug signing key.

[安装说明与校验值](downloads/README.md) · [抓包使用说明](CAPTURE.md)

### Feature

Feature of [Clash.Meta](https://github.com/MetaCubeX/Clash.Meta)

Local HTTP/HTTPS capture (including HTTPS HTTP/2), with application, domain and path allowlists. See [capture setup and limitations](CAPTURE.md).

[<img src="https://fdroid.gitlab.io/artwork/badge/get-it-on.png"
     alt="Get it on F-Droid"
     height="80">](https://f-droid.org/packages/com.github.metacubex.clash.meta/)

### Requirement

- Android 5.0+ (minimum)
- Android 7.0+ (recommend)
- `armeabi-v7a` , `arm64-v8a`, `x86` or `x86_64` Architecture

### Build

Build the named capture application with `./gradlew :app:assembleCaptureRelease`. This flavor uses the fixed package ID `com.torin.clash.capture` and display name `T-Agent Capture`. The Alpha and Meta variants remain available; the optional package override below applies to those variants.

GitHub Debug and Pre-Release builds target T-Agent Capture. The workflows use the explicit Go cache for the nested Go modules. Pre-Release always uploads test APKs as workflow artifacts. Without signing secrets it uses the debug key and skips publishing a GitHub release; these test APKs are not signed with the key used for the device-tested APK above. To publish stable signed releases, configure `SIGNING_STORE_PASSWORD`, `SIGNING_KEY_ALIAS`, and `SIGNING_KEY_PASSWORD` for the tracked `release.keystore`. All three must be present together. The manual Release workflow requires these secrets before changing versions or starting the build. Signing settings are never printed to logs.

1. Update submodules

   ```bash
   git submodule update --init --recursive
   ```

2. Install **OpenJDK 11**, **Android SDK**, **CMake** and **Golang**

3. Create `local.properties` in project root with

   ```properties
   sdk.dir=/path/to/android-sdk
   ```

4. (Optional) Custom app package name. Add the following configuration to `local.properties`.

   ```properties
   # config your ownn applicationId, or it will be 'com.github.metacubex.clash'
   custom.application.id=com.my.compile.clash
   # remove application id suffix, or the applicaion id will be 'com.github.metacubex.clash.alpha'
   remove.suffix=true

5. Create `signing.properties` in project root with

   ```properties
   keystore.path=/path/to/keystore/file
   keystore.password=<key store password>
   key.alias=<key alias>
   key.password=<key password>
   ```

6. Build

   ```bash
   ./gradlew app:assembleAlphaRelease
   ```

### Automation

APP package name is `com.github.metacubex.clash.meta`

- Toggle Clash.Meta service status
  - Send intent to activity `com.github.kr328.clash.ExternalControlActivity` with action `com.github.metacubex.clash.meta.action.TOGGLE_CLASH`
- Start Clash.Meta service
  - Send intent to activity `com.github.kr328.clash.ExternalControlActivity` with action `com.github.metacubex.clash.meta.action.START_CLASH`
- Stop Clash.Meta service
  - Send intent to activity `com.github.kr328.clash.ExternalControlActivity` with action `com.github.metacubex.clash.meta.action.STOP_CLASH`
- Import a profile
  - URL Scheme `clash://install-config?url=<encoded URI>` or `clashmeta://install-config?url=<encoded URI>`

### Contribution and Project Maintenance

#### Meta Kernel

- CMFA uses the kernel from `android-real` branch under `MetaCubeX/Clash.Meta`, which is a merge of the main `Alpha` branch and `android-open`.
  - If you want to contribute to the kernel, make PRs to `Alpha` branch of the Meta kernel repository.
  - If you want to contribute Android-specific patches to the kernel, make PRs to  `android-open` branch of the Meta kernel repository.

#### Maintenance

- When `MetaCubeX/Clash.Meta` kernel is updated to a new version, the `Update Dependencies` actions in this repo will be triggered automatically.
  - It will pull the new version of the meta kernel, update all the golang dependencies, and create a PR without manual intervention.
  - If there is any compile error in PR, you need to fix it before merging. Alternatively, you may merge the PR directly.
- Manually triggering `Build Pre-Release` actions will compile and publish a `PreRelease` version.
- Manually triggering `Build Release` actions will compile, tag and publish a `Release` version.
  - You must fill the blank `Release Tag` with the tag you want to release in the format of `v1.2.3`.
  - `versionName` and `versionCode` in `build.gradle.kts` will be automatically bumped to the tag you filled above.
