## Clash Meta for Android

Automatic profile updates use the interval set in each profile's properties (minimum 15 minutes; clear it to disable updates). In App Settings, enable **Pause automatic updates at night** and choose the start and end hour for a device-local quiet window. Updates due during that window run when it ends. Manual profile updates remain available. Matching start and end hours disable the window.

Version `2.11.35.capture-scheduling.Alpha` (211042) combines these settings with the HTTP/HTTPS capture feature, capture allowlists, diagnostics, and cURL script export. It uses the capture application's existing package ID, `com.github.metacubex.clash.alpha`. Updating an existing installation requires the same signing key.

A Graphical user interface of [Clash.Meta](https://github.com/MetaCubeX/Clash.Meta) for Android

### APK download

[Download the latest APK (ARM64, 211042)](https://github.com/TorinMars/ClashMetaForAndroid/raw/refs/heads/main/downloads/cmfa-2.11.35-capture-scheduling-arm64-v8a.apk)

Version: `2.11.35.capture-scheduling.Alpha`. Includes HTTP/HTTPS capture, independent capture and proxy routing scopes, cURL script export, configurable profile update intervals, and overnight quiet hours. This APK uses the current local debug signing key; it cannot update an earlier APK signed with a different key. Uninstalling an earlier installation clears its configuration and capture CA.

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
