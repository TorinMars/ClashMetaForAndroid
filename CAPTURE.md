# HTTP / HTTPS 抓包

入口：设置 → HTTP / HTTPS 抓包。

1. 导入配置并启动 Clash 的 VPN 模式。
2. 选择应用白名单，填写域名、path 白名单（每行一项）。
3. HTTPS：导出本机生成的 CA 公共证书，在 Android 安全设置中安装，并确保目标应用信任它。
4. 勾选启用抓包 / 解密 HTTPS，点击“保存并应用”，再从目标应用发起新请求。
5. 点击记录查看请求头、响应头、正文和错误；可将单条记录导出为 JSON。

## 规则

- 同类白名单取并集，不同类别取交集。空列表表示该类不限。
- 应用按 Android UID 匹配。共享 UID 的应用无法隔离；无法识别 UID 的连接不会匹配非空应用白名单。名单内应用被卸载或 UID 改变时停止抓包，防止 UID 重用导致误抓。
- 域名忽略大小写；`example.com` 精确匹配，`*.example.com` 只匹配子域名，不包括根域名。国际化域名使用 Punycode。
- path 区分大小写，不含 query。`/api` 精确匹配；`/api/*` 匹配前缀 `/api/`。按 URL 中的转义路径匹配，不做 URL 解码或路径归一化。
- HTTPS 先按应用、SNI 域名决定是否解密，再按 path 决定是否保存记录。未匹配 path 的请求仍会经解密代理正常转发。
- 未匹配应用/域名的流量按原有 Clash 路由放行。已匹配的上游请求也经过原有 Clash 路由；校验上游 TLS 证书，不跳过校验。
- 设置变更/停止抓包会关闭当前被检查的连接；新规则从新连接生效。开启抓包前已存在的连接不会被自动接管。

## 支持范围与限制

若目标应用遇到 TLS 握手失败，可开启“HTTPS 兼容模式（仅 TLS 1.2）”，保存并应用，再重启目标应用测试。该开关仅限制应用到抓包代理的 TLS 版本，不改变上游 TLS 校验或现有 CA；关闭后恢复自动协商。错误信息包含目标域名、UID 和当前模式。此模式用于兼容性排查，尚未确认能解决 KIM / Android 17 上的问题。

支持 TCP 80 上的 HTTP/1.x，以及 TCP 443 上的 HTTPS HTTP/1.x、HTTP/2。HTTP/2 可保留 trailers（例如 gRPC 状态），但不解释 protobuf 数据。WebSocket 仅记录升级握手，不解析帧。

HTTP/3/QUIC、其他端口、非 HTTP ALPN 和没有 SNI 的 HTTPS 直接放行，不记录。若目标应用优先使用 QUIC，需要在受测应用中关闭 HTTP/3 后重新建立连接。

Android 7+ 的应用默认可能不信任用户安装的 CA，证书固定、双向 TLS 和 ECH 等场景不保证可用。本功能不绕过目标应用的证书固定；应在自己的测试应用中配置调试 CA 信任。证书不被信任时，匹配域名的 HTTPS 请求可能失败，界面会显示握手错误。

参考：[Android 网络安全配置](https://developer.android.com/privacy-and-security/security-config)。

## 数据与资源

### 无请求时的诊断

抓包页提供“一键复制诊断日志”。保存设置后重启目标应用、复现一次，再返回抓包页复制并粘贴反馈。复制前不要清空记录或再次保存设置，这两种操作都会重置诊断。

日志包含构建/Android 版本、设备型号、已保存的 VPN 网络设置、所选应用包名/版本/UID、抓包条件、TCP 新连接计数、UDP 数据包计数，以及最近 200 条连接阶段事件（页面只显示最近 20 条）。事件包含端口、目标 IP/域名、TLS 探测与握手信息、跳过原因和请求完成状态，不含 HTTP 请求头/正文、CA 私钥或代理凭据。仅保存在内存中，点击复制后进入系统剪贴板。

诊断只覆盖开启抓包后的 TUN 新 TCP 连接和 UDP 数据包，不覆盖已有 TCP 连接或系统 HTTP 代理入口。UDP 总数包含所有应用，UID 每秒最多抽样一次，不能据此断定所选应用未发 UDP；UDP 443 也不一定是 QUIC。网络设置为保存值，需重启 VPN 才能确保生效。诊断没有流量时应继续检查接管路径，不能认定 KIM 没有发请求。

- 默认关闭，不随 VPN/进程重启自动启用。
- CA 在设备上随机生成；私钥保存在应用私有目录 `files/clash/capture/authority.pem`（0600），不导出、不纳入应用备份。每台安装独立 CA。
- 仅内存保留最近 100 条已完成请求。每个请求/响应正文保留前 8 KiB，超出标记截断；转发流量不截断。
- 头部最多约 8 KiB，列表 URL 最多约 2 KiB。最多同时检查 64 条连接；超过并发限制的连接直接放行。
- 正文按原始字节 Base64 导出；界面可显示 UTF-8 和有界 gzip 预览，二进制/其他压缩正文显示 Base64。
- 清空操作会抑制清空前仍在处理的请求重新出现。清空后的新请求仍正常记录。
- 记录可能包含 Cookie、Authorization、个人数据；只在用户选择导出时写出所选记录。完整抓包内容不会写入普通日志。

## 实现与验证

`native/capture` 实现 CA、筛选、TLS/HTTP 代理及有界记录；`native/tun/capture.go` 包装 TUN 入站，通过 `net.Pipe` 把上游请求交回原来的 Mihomo tunnel。界面通过现有 Binder 和 JNI 桥接调用，不开放网络管理端口。

测试命令（先加载工作区 `build-env.sh`）：

```bash
cd core/src/main/golang
go test -race -timeout 90s ./native/capture
```

覆盖 HTTP/HTTPS 和 HTTP/2 转发、path 筛选、HTTP/TLS 原字节放行、UID 筛选、上游不可信证书拒绝、CA 持久性和私钥权限、正文/记录上限、清空并发及停止连接。

APK 验证使用 `./gradlew --no-daemon app:assembleAlphaRelease --console=plain --max-workers=4`。真机 VPN/CA 安装和不同 OEM 下的 UID 查询仍需实机验证。
