package tun

import (
	"context"
	"net"
	"net/netip"
	"strconv"

	"cfa/native/app"
	"cfa/native/capture"
	"github.com/metacubex/mihomo/common/utils"
	"github.com/metacubex/mihomo/component/ca"
	"github.com/metacubex/mihomo/component/resolver"
	C "github.com/metacubex/mihomo/constant"
	P "github.com/metacubex/mihomo/constant/provider"
)

type captureTunnel struct {
	C.Tunnel
	providers P.Tunnel
	routing   capture.Routing
}

func (t *captureTunnel) routed(metadata *C.Metadata) (*C.Metadata, int) {
	uid := -1
	if metadata.RawSrcAddr != nil && metadata.RawDstAddr != nil {
		uid = app.QuerySocketUid(metadata.RawSrcAddr, metadata.RawDstAddr)
	}
	if proxy := t.routing.Proxy(uid); proxy != "" {
		copy := *metadata
		copy.SpecialProxy = proxy
		metadata = &copy
	}
	return metadata, uid
}

func (t *captureTunnel) Providers() map[string]P.ProxyProvider    { return t.providers.Providers() }
func (t *captureTunnel) RuleProviders() map[string]P.RuleProvider { return t.providers.RuleProviders() }
func (t *captureTunnel) RuleUpdateCallback() *utils.Callback[P.RuleProvider] {
	return t.providers.RuleUpdateCallback()
}
func (t *captureTunnel) HandleUDPPacket(packet C.UDPPacket, metadata *C.Metadata) {
	if t.routing.Mode != "" {
		metadata, _ = t.routed(metadata)
	}
	capture.Default.ObserveUDP(int(metadata.DstPort), func() int {
		if metadata.RawSrcAddr != nil && metadata.RawDstAddr != nil {
			return app.QuerySocketUid(metadata.RawSrcAddr, metadata.RawDstAddr)
		}
		return -1
	})
	t.Tunnel.HandleUDPPacket(packet, metadata)
}
func (t *captureTunnel) HandleTCPConn(conn net.Conn, metadata *C.Metadata) {
	var uid int
	if capture.Default.Enabled() || t.routing.Mode != "" {
		metadata, uid = t.routed(metadata)
	}
	if !capture.Default.Enabled() {
		t.Tunnel.HandleTCPConn(conn, metadata)
		return
	}
	original := *metadata
	dial := func(ctx context.Context, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		n, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return nil, err
		}
		meta := original
		meta.Host = host
		meta.DstPort = uint16(n)
		// Keep a real destination IP selected by the application; fake IPs must
		// be resolved by the normal Mihomo path using the inspected hostname.
		if resolver.IsFakeIP(meta.DstIP) {
			meta.DstIP = netip.Addr{}
			meta.DNSMode = C.DNSFakeIP
		}
		if ip, err := netip.ParseAddr(host); err == nil {
			meta.DstIP = ip
			meta.Host = ""
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		local, remote := net.Pipe()
		go t.Tunnel.HandleTCPConn(remote, &meta)
		return local, nil
	}
	if !capture.Default.Handle(conn, uid, int(metadata.DstPort), dial, func(c net.Conn) { t.Tunnel.HandleTCPConn(c, metadata) }, ca.GetCertPool(), metadata.DstIP.String(), metadata.Host) {
		t.Tunnel.HandleTCPConn(conn, metadata)
	}
}
