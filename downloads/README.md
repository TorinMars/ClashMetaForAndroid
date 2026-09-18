# 抓包测试版 APK

[下载 ARM64 APK](https://github.com/TorinMars/ClashMetaForAndroid/raw/refs/heads/http-capture/downloads/cmfa-2.11.34-capture-curl-arm64-v8a.apk)

版本：`2.11.34.capture-curl.Alpha`（211038）。长按抓包记录弹出菜单，选择“复制为 cURL”，成功后显示 Toast。命令适用于 macOS/Linux 等 POSIX shell，包含方法、URL、请求头和已保存的正文；二进制正文用 base64 管道传入。请求被截断时提示无法生成完整命令。保留 Path 正则、诊断日志复制和 TLS 1.2 兼容模式，覆盖安装旧测试版可保留 CA。

适用于 ARM64 Android 手机，最低 Android 5.0。安装后进入“设置 → HTTP / HTTPS 抓包”。

此文件由本分支抓包功能源码构建，使用本地 debug 签名。它无法覆盖安装签名不同的官方版本。

- HTTP / HTTPS（含 HTTPS HTTP/2）。
- 应用、域名、path 白名单；不同类别同时满足才记录。
- CA 公共证书导出、请求/响应详情、单条 JSON 导出。
- HTTPS 解密需要目标应用信任抓包 CA；不绕过证书固定，不抓取 HTTP/3。
- 完整 APK 构建、签名校验及 Go 竞态测试通过；尚未完成真机验证。

详情见 [使用说明](../CAPTURE.md)。文件 SHA-256 见 [SHA256SUMS](SHA256SUMS)。
