package config

import "testing"

// TestProxyCapable pins the capability matrix. It is the contract the server's
// save-time validation, the agent's monitor evaluation, and the console's proxy
// picker all read, so every cell is asserted explicitly — a silent change here
// would let the console offer a combination the agent can only refuse.
func TestProxyCapable(t *testing.T) {
	cases := []struct {
		name      string
		kind      string
		params    ProbeParams
		proxyType string
		want      bool
	}{
		// http / tcp tunnel over every type.
		{"http via socks5", "http", ProbeParams{}, ProxyTypeSOCKS5, true},
		{"http via http", "http", ProbeParams{}, ProxyTypeHTTP, true},
		{"http via wireguard", "http", ProbeParams{}, ProxyTypeWireGuard, true},
		{"tcp via socks5", "tcp", ProbeParams{}, ProxyTypeSOCKS5, true},
		{"tcp via http", "tcp", ProbeParams{}, ProxyTypeHTTP, true},
		{"tcp via wireguard", "tcp", ProbeParams{}, ProxyTypeWireGuard, true},

		// DNS. SOCKS5 relays UDP via UDP ASSOCIATE, so every resolver protocol works —
		// including the datagram ones. HTTP has only CONNECT, so it is limited to the
		// stream-framed protocols. Every case must also name a resolver endpoint: see
		// proxiedDNSHasEndpoint.
		{"dns udp via socks5", "dns", ProbeParams{ResolverProtocol: "udp", ResolverServer: "1.1.1.1"}, ProxyTypeSOCKS5, true},
		{"dns tcp via socks5", "dns", ProbeParams{ResolverProtocol: "tcp", ResolverServer: "1.1.1.1"}, ProxyTypeSOCKS5, true},
		{"dns dot via socks5", "dns", ProbeParams{ResolverProtocol: "dot", ResolverServer: "1.1.1.1"}, ProxyTypeSOCKS5, true},
		{"dns doh via socks5", "dns", ProbeParams{ResolverProtocol: "doh", ResolverServer: "https://dns.example/dns-query"}, ProxyTypeSOCKS5, true},
		{"dns doh via http", "dns", ProbeParams{ResolverProtocol: "doh", ResolverServer: "https://dns.example/dns-query"}, ProxyTypeHTTP, true},
		{"dns tcp via http", "dns", ProbeParams{ResolverProtocol: "tcp", ResolverServer: "1.1.1.1"}, ProxyTypeHTTP, true},
		{"dns udp via http", "dns", ProbeParams{ResolverProtocol: "udp", ResolverServer: "1.1.1.1"}, ProxyTypeHTTP, false},
		{"dns udp via wireguard", "dns", ProbeParams{ResolverProtocol: "udp", ResolverServer: "1.1.1.1"}, ProxyTypeWireGuard, true},
		// Case and surrounding whitespace must not change the verdict — the value arrives
		// from a form field. Asserted on HTTP, the type that actually branches on it.
		{"dns DoT uppercase via http", "dns", ProbeParams{ResolverProtocol: " DoT ", ResolverServer: "1.1.1.1"}, ProxyTypeHTTP, true},

		// The SYSTEM resolver has no address on the wire, so no proxy or tunnel can carry
		// it. Left capable, such a monitor would resolve off the host and report success
		// while the pinned egress was down — the fail-open the pin exists to prevent.
		{"dns system resolver via socks5", "dns", ProbeParams{}, ProxyTypeSOCKS5, false},
		{"dns system resolver via http", "dns", ProbeParams{}, ProxyTypeHTTP, false},
		{"dns system resolver via wireguard", "dns", ProbeParams{}, ProxyTypeWireGuard, false},
		// Same rule when a protocol is named but the endpoint is not (a blank field).
		{"dns udp without a resolver via socks5", "dns", ProbeParams{ResolverProtocol: "udp"}, ProxyTypeSOCKS5, false},
		{"dns tcp without a resolver via http", "dns", ProbeParams{ResolverProtocol: "tcp"}, ProxyTypeHTTP, false},
		{"dns whitespace resolver via socks5", "dns", ProbeParams{ResolverProtocol: "udp", ResolverServer: "  "}, ProxyTypeSOCKS5, false},

		// NAT/STUN. Same split: SOCKS5 carries every transport, HTTP only the
		// stream-framed ones ("" defaults to udp).
		{"nat default via socks5", "nat", ProbeParams{}, ProxyTypeSOCKS5, true},
		{"nat udp via socks5", "nat", ProbeParams{NATTransport: "udp"}, ProxyTypeSOCKS5, true},
		{"nat dtls via socks5", "nat", ProbeParams{NATTransport: "dtls"}, ProxyTypeSOCKS5, true},
		{"nat tcp via socks5", "nat", ProbeParams{NATTransport: "tcp"}, ProxyTypeSOCKS5, true},
		{"nat tls via socks5", "nat", ProbeParams{NATTransport: "tls"}, ProxyTypeSOCKS5, true},
		{"nat default via http", "nat", ProbeParams{}, ProxyTypeHTTP, false},
		{"nat udp via http", "nat", ProbeParams{NATTransport: "udp"}, ProxyTypeHTTP, false},
		{"nat dtls via http", "nat", ProbeParams{NATTransport: "dtls"}, ProxyTypeHTTP, false},
		{"nat tcp via http", "nat", ProbeParams{NATTransport: "tcp"}, ProxyTypeHTTP, true},
		{"nat tls via http", "nat", ProbeParams{NATTransport: "tls"}, ProxyTypeHTTP, true},
		{"nat udp via wireguard", "nat", ProbeParams{NATTransport: "udp"}, ProxyTypeWireGuard, true},
		{"nat dtls via wireguard", "nat", ProbeParams{NATTransport: "dtls"}, ProxyTypeWireGuard, true},

		// ICMP is the one kind NEITHER relay can forward — no SOCKS5 command and no
		// CONNECT semantics exist for it — so it stays tunnel-only even though SOCKS5
		// does carry UDP.
		{"icmp via socks5", "icmp", ProbeParams{}, ProxyTypeSOCKS5, false},
		{"icmp via http", "icmp", ProbeParams{}, ProxyTypeHTTP, false},
		{"icmp via wireguard", "icmp", ProbeParams{}, ProxyTypeWireGuard, true},

		// Local / server-side anchors are never proxied.
		{"gateway via socks5", "gateway", ProbeParams{}, ProxyTypeSOCKS5, false},
		{"gateway via wireguard", "gateway", ProbeParams{}, ProxyTypeWireGuard, false},
		{"host via wireguard", "host", ProbeParams{}, ProxyTypeWireGuard, false},

		// Unknown inputs are refused, never assumed capable.
		{"unknown proxy type", "http", ProbeParams{}, "shadowsocks", false},
		{"unknown kind", "smtp", ProbeParams{}, ProxyTypeWireGuard, false},
		{"empty proxy type", "http", ProbeParams{}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ProxyCapable(c.kind, c.params, c.proxyType); got != c.want {
				t.Fatalf("ProxyCapable(%q, %+v, %q) = %v, want %v", c.kind, c.params, c.proxyType, got, c.want)
			}
		})
	}
}

// TestAnyProxyCapable covers the console's "should the proxy picker exist at all"
// decision. A kind that no proxy type can carry must not grow a control the user
// could only leave empty.
func TestAnyProxyCapable(t *testing.T) {
	cases := []struct {
		name   string
		kind   string
		params ProbeParams
		want   bool
	}{
		{"http", "http", ProbeParams{}, true},
		{"tcp", "tcp", ProbeParams{}, true},
		// UDP DNS still qualifies: WireGuard can carry it, so the picker shows with
		// only the tunnel offered.
		{"dns udp (wireguard only)", "dns", ProbeParams{ResolverProtocol: "udp", ResolverServer: "1.1.1.1"}, true},
		// With no resolver endpoint no transport can carry it, so the picker is hidden.
		{"dns system resolver", "dns", ProbeParams{}, false},
		{"nat udp (wireguard only)", "nat", ProbeParams{}, true},
		{"icmp (wireguard only)", "icmp", ProbeParams{}, true},
		{"gateway", "gateway", ProbeParams{}, false},
		{"host", "host", ProbeParams{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AnyProxyCapable(c.kind, c.params); got != c.want {
				t.Fatalf("AnyProxyCapable(%q, %+v) = %v, want %v", c.kind, c.params, got, c.want)
			}
		})
	}
}

func TestDNSModeOrDefault(t *testing.T) {
	cases := []struct {
		name string
		spec ProxySpec
		want string
	}{
		{"unset defaults to local", ProxySpec{Type: ProxyTypeSOCKS5}, ProxyDNSLocal},
		{"explicit local", ProxySpec{Type: ProxyTypeSOCKS5, DNSMode: ProxyDNSLocal}, ProxyDNSLocal},
		{"explicit remote", ProxySpec{Type: ProxyTypeSOCKS5, DNSMode: ProxyDNSRemote}, ProxyDNSRemote},
		{"garbage falls back to local", ProxySpec{Type: ProxyTypeHTTP, DNSMode: "whatever"}, ProxyDNSLocal},
		// A tunnel resolves in-tunnel; there is no proxy-side alternative to select,
		// so a stored "remote" must not be honored.
		{"wireguard ignores remote", ProxySpec{Type: ProxyTypeWireGuard, DNSMode: ProxyDNSRemote}, ProxyDNSLocal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.spec.DNSModeOrDefault(); got != c.want {
				t.Fatalf("DNSModeOrDefault() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestProxyTimeoutAndMTUDefaults(t *testing.T) {
	if got := (ProxySpec{}).ProxyConnectTimeout(); got != DefaultProxyConnectTimeout {
		t.Fatalf("unset connect timeout = %v, want %v", got, DefaultProxyConnectTimeout)
	}
	if got := (ProxySpec{ConnectTimeoutMs: 250}).ProxyConnectTimeout(); got.Milliseconds() != 250 {
		t.Fatalf("configured connect timeout = %v, want 250ms", got)
	}
	if got := (ProxySpec{}).WireGuardMTU(); got != DefaultWireGuardMTU {
		t.Fatalf("unset MTU = %d, want %d", got, DefaultWireGuardMTU)
	}
	if got := (ProxySpec{WGMTU: 1280}).WireGuardMTU(); got != 1280 {
		t.Fatalf("configured MTU = %d, want 1280", got)
	}
}

func TestKnownProxyType(t *testing.T) {
	for _, ty := range []string{ProxyTypeSOCKS5, ProxyTypeHTTP, ProxyTypeWireGuard} {
		if !KnownProxyType(ty) {
			t.Fatalf("KnownProxyType(%q) = false, want true", ty)
		}
	}
	for _, ty := range []string{"", "socks4", "SOCKS5", "wg", "*"} {
		if KnownProxyType(ty) {
			t.Fatalf("KnownProxyType(%q) = true, want false", ty)
		}
	}
}
