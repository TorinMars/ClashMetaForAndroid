# Capture APK builds

## T-Agent Capture 2.11.36 (211043)

[Download the ARM64 APK](t-agent-capture-2.11.36-arm64-v8a.apk).

Application and launcher name: **T-Agent Capture**. Package ID: `com.torin.clash.capture`. This build combines packet capture and subscription scheduling under a separate app identity, so it can be installed alongside earlier Alpha and Meta builds. Profiles, settings, and the capture CA belong to the new installation and need separate setup.

Build target: `:app:assembleCaptureRelease`. The APK is signed with the current local debug key. Its checksum is listed in [SHA256SUMS](SHA256SUMS).

Verification: Capture variant service unit tests (2 passed), the Capture release build and lint checks, APK package/label/signature checks, and installation and launch on an ARM64 Android phone. The earlier Alpha capture app remained installed. Captured traffic was not exercised during this installation check.

## 2.11.35.capture-scheduling.Alpha (211042)

[Download the ARM64 APK](cmfa-2.11.35-capture-scheduling-arm64-v8a.apk).

This build combines HTTP/HTTPS capture, allowlists, diagnostics, cURL export, and independent capture routing with subscription update intervals and device-local quiet hours. Set each profile's interval in its properties; configure the quiet window in App Settings. Manual updates remain available during quiet hours.

Package: `com.github.metacubex.clash.alpha`. This build uses the current local debug signing key. Updating an existing installation requires a matching certificate; an older capture APK built with another debug key cannot be updated by this file. Uninstalling the old app removes its profiles, settings, and capture CA.

Verification: Alpha service unit tests, Go capture tests with the race detector, and the Alpha release build. The APK's SHA-256 is listed in [SHA256SUMS](SHA256SUMS).

## Earlier builds

新增“一键复制全部请求为 Shell 脚本”：当前保留的最多 100 条请求按时间顺序组成 cURL 脚本，保存为 `requests.sh` 后执行 `sh requests.sh`。不完整记录注明并跳过；脚本过大时直接保存文件。复制时不会执行请求。

本版将抓包名单与分应用路由名单分开：额外接管的抓包应用强制直连，原代理范围保持原规则。保存或停止抓包时自动重建接管范围，原名单不变。仍需本地 VPN；策略测试通过，真机接管和恢复行为待验证。

[下载 ARM64 APK](https://github.com/TorinMars/ClashMetaForAndroid/raw/refs/heads/http-capture/downloads/cmfa-2.11.34-curl-script-arm64-v8a.apk)

版本：`2.11.34.curl-script.Alpha`（211041）。在“设置 → 网络 → 访问控制应用包列表”的右上角菜单加入醒目的分应用路由导入、导出入口。导出复制 JSON（路由模式与包名），导入恢复名单与模式；兼容旧版每行一个包名的文本。未安装应用的包名保留，已选系统应用不再被隐藏；无效数据不会清空原配置。操作后显示提示，返回上一页保存并按需重启 VPN。保留抓包、Path 正则、复制 cURL 和诊断功能，覆盖安装旧测试版可保留 CA。

适用于 ARM64 Android 手机，最低 Android 5.0。安装后进入“设置 → HTTP / HTTPS 抓包”。

此文件由本分支抓包功能源码构建，使用本地 debug 签名。它无法覆盖安装签名不同的官方版本。

- HTTP / HTTPS（含 HTTPS HTTP/2）。
- 应用、域名、path 白名单；不同类别同时满足才记录。
- CA 公共证书导出、请求/响应详情、单条 JSON 导出。
- HTTPS 解密需要目标应用信任抓包 CA；不绕过证书固定，不抓取 HTTP/3。
- 完整 APK 构建、签名校验及 Go 竞态测试通过；尚未完成真机验证。

详情见 [使用说明](../CAPTURE.md)。文件 SHA-256 见 [SHA256SUMS](SHA256SUMS)。
