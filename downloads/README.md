# 抓包测试版 APK

[下载 ARM64 APK](https://github.com/TorinMars/ClashMetaForAndroid/raw/refs/heads/http-capture/downloads/cmfa-2.11.34-capture-regex-arm64-v8a.apk)

版本：`2.11.34.capture-regex.Alpha`（211037）。新增 Path 正则匹配：在 Path 白名单填写 `re:(?i)scan` 即可记录路径中包含 scan 的请求（忽略大小写，不匹配查询参数）。每行一项，可与精确路径和末尾 `*` 混用，满足任意一项即可。保存并应用后生效；无效正则会报错并保留旧配置。保留一键复制诊断日志和 TLS 1.2 兼容模式，覆盖安装旧测试版可保留 CA。

适用于 ARM64 Android 手机，最低 Android 5.0。安装后进入“设置 → HTTP / HTTPS 抓包”。

此文件由本分支抓包功能源码构建，使用本地 debug 签名。它无法覆盖安装签名不同的官方版本。

- HTTP / HTTPS（含 HTTPS HTTP/2）。
- 应用、域名、path 白名单；不同类别同时满足才记录。
- CA 公共证书导出、请求/响应详情、单条 JSON 导出。
- HTTPS 解密需要目标应用信任抓包 CA；不绕过证书固定，不抓取 HTTP/3。
- 完整 APK 构建、签名校验及 Go 竞态测试通过；尚未完成真机验证。

详情见 [使用说明](../CAPTURE.md)。文件 SHA-256 见 [SHA256SUMS](SHA256SUMS)。
