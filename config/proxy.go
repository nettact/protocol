package config

import (
	"strings"
	"time"
)

// Egress proxies for monitoring targets.
//
// A ProxySpec is a site-scoped, named, reusable egress path a target may be
// pinned to. Targets reference one by ProbeTarget.ProxyID; the server pushes the
// referenced specs down inside DesiredState so the agent never stores proxy
// config of its own.
//
// Two shapes hide behind one type:
//
//   - socks5 / http are real proxies: the agent dials the proxy and asks it to
//     relay to the target. SOCKS5 relays TCP via CONNECT and UDP via UDP ASSOCIATE
//     (RFC 1928 §7); HTTP has only CONNECT, so it is TCP-only by protocol.
//   - wireguard is NOT a proxy — it is a tunnel. The agent runs a userspace
//     WireGuard device and dials the target from inside it. That is why it is the
//     only type that can carry ICMP (see ProxyCapable) — neither relay protocol
//     has any notion of forwarding an ICMP echo.
//
// Everything here is fail-closed by contract: a target naming a proxy the agent
// cannot build, or cannot use for its kind, must NOT fall back to a direct dial.
// A silent fallback would leak the real egress IP to the target and make a
// "reachable" verdict meaningless.

// Proxy types.
const (
	ProxyTypeSOCKS5    = "socks5"
	ProxyTypeHTTP      = "http"
	ProxyTypeWireGuard = "wireguard"
)

// Proxy dial/handshake defaults.
//
// DefaultProxyConnectTimeout bounds reaching the proxy and completing its
// handshake, SEPARATELY from the probe's own timeout: the two failures are
// different diagnoses, and a proxy that hangs must not consume the whole probe
// budget and then be reported as a target timeout.
//
// DefaultWireGuardMTU is the conventional WireGuard tunnel MTU (1500 minus the
// 60-byte IPv6+UDP+WireGuard worst-case overhead).
const (
	DefaultProxyConnectTimeout = 5 * time.Second
	DefaultWireGuardMTU        = 1420
)

// ProxyConnectTimeout is the spec's proxy-handshake budget, or the default when
// unset.
func (p ProxySpec) ProxyConnectTimeout() time.Duration {
	if p.ConnectTimeoutMs > 0 {
		return time.Duration(p.ConnectTimeoutMs) * time.Millisecond
	}
	return DefaultProxyConnectTimeout
}

// WireGuardMTU is the spec's tunnel MTU, or the default when unset.
func (p ProxySpec) WireGuardMTU() int {
	if p.WGMTU > 0 {
		return p.WGMTU
	}
	return DefaultWireGuardMTU
}

// Proxy DNS resolution modes — WHERE the target hostname is resolved.
//
// ProxyDNSLocal (the default) resolves on the agent, so the agent's target-access
// policy still vets the concrete address and the resolved literal IP is what gets
// handed to the proxy. That keeps the DNS-rebinding defense intact: the name is
// resolved exactly once and the address the policy approved is the address the
// proxy connects to.
//
// ProxyDNSRemote hands the hostname to the proxy instead. It is the only mode
// that works when the target name only resolves correctly on the proxy's side
// (split-horizon DNS), but the agent can no longer vet the final address — only
// the name, pre-resolution. Callers must treat it as the weaker mode it is.
const (
	ProxyDNSLocal  = "local"
	ProxyDNSRemote = "remote"
)

// ProxySpec is one egress proxy pushed to the agent. Credentials travel in the
// clear inside the already-authenticated agent WebSocket channel; they are never
// written to agent disk and never echoed back by the server's read APIs.
type ProxySpec struct {
	ID   string `json:"id"`             // stable server-side proxy id (proxies.id)
	Name string `json:"name,omitempty"` // human-friendly display name
	Type string `json:"type"`           // ProxyType* — socks5 | http | wireguard
	// ConfigSerial is this proxy's material config generation. The agent keys its
	// built dialer on (id, config_serial) so an edited proxy is torn down and
	// rebuilt rather than kept alive on stale credentials or a stale connection.
	ConfigSerial int `json:"config_serial,omitempty"`

	// socks5 / http.
	Host             string `json:"host,omitempty"`               // proxy host or IP
	Port             int    `json:"port,omitempty"`               // proxy port
	Username         string `json:"username,omitempty"`           // optional auth
	Password         string `json:"password,omitempty"`           // optional auth (secret)
	DNSMode          string `json:"dns_mode,omitempty"`           // ProxyDNS* ("" = local)
	ConnectTimeoutMs int    `json:"connect_timeout_ms,omitempty"` // proxy-handshake budget; 0 = DefaultProxyConnectTimeout

	// wireguard (userspace tunnel).
	WGPrivateKey       string `json:"wg_private_key,omitempty"`     // this peer's private key, base64 (secret)
	WGPeerPublicKey    string `json:"wg_peer_public_key,omitempty"` // remote peer public key, base64
	WGPresharedKey     string `json:"wg_preshared_key,omitempty"`   // optional PSK, base64 (secret)
	WGEndpoint         string `json:"wg_endpoint,omitempty"`        // remote peer host:port
	WGAllowedIPs       string `json:"wg_allowed_ips,omitempty"`     // CSV of CIDRs routed into the tunnel
	WGLocalAddrs       string `json:"wg_local_addrs,omitempty"`     // CSV of this peer's in-tunnel addresses
	WGDNS              string `json:"wg_dns,omitempty"`             // CSV of in-tunnel resolvers
	WGMTU              int    `json:"wg_mtu,omitempty"`             // tunnel MTU; 0 = DefaultWireGuardMTU
	WGKeepaliveSeconds int    `json:"wg_keepalive_seconds,omitempty"`
}

// DNSModeOrDefault returns the spec's DNS mode, defaulting to local. WireGuard
// resolves inside the tunnel and has no proxy-side alternative, so it always
// reports local regardless of what was stored.
func (p ProxySpec) DNSModeOrDefault() string {
	if p.Type == ProxyTypeWireGuard {
		return ProxyDNSLocal
	}
	if p.DNSMode == ProxyDNSRemote {
		return ProxyDNSRemote
	}
	return ProxyDNSLocal
}

// ProxyCapable reports whether a probe of this kind, with these params, can run
// through a proxy of this type.
//
// This is the ONE source of truth for the capability matrix, shared by the
// server's save-time validation, the agent's monitor evaluation, and the
// console's proxy picker, so the three can never disagree about what is
// offerable, savable, and runnable.
//
// The matrix follows from what each transport can actually carry:
//
//	kind     | socks5            | http              | wireguard
//	---------+-------------------+-------------------+----------
//	http     | yes               | yes               | yes
//	tcp      | yes               | yes               | yes
//	dns      | yes               | stream protos only| yes
//	nat      | yes               | tcp/tls only      | yes
//	icmp     | no                | no                | yes
//	gateway  | no                | no                | no
//	host     | no                | no                | no
//
// The three columns differ by what the transport can forward, not by convenience:
//
//   - SOCKS5 forwards TCP (CONNECT) AND UDP (UDP ASSOCIATE), so every datagram
//     probe except ICMP works. It has no ICMP relay at all — the protocol has no
//     command for it — so ping stays tunnel-only.
//   - HTTP has only CONNECT, which tunnels a TCP byte stream. Plain-UDP DNS and
//     STUN over udp/dtls therefore cannot traverse it, and are refused rather than
//     offered and then failed every cycle.
//   - WireGuard carries raw IP, so it carries everything the agent probes.
//
// A gateway probe targets the local first hop, where an egress proxy has no
// meaning at all, and host is a server-side anchor that is never pushed.
func ProxyCapable(kind string, p ProbeParams, proxyType string) bool {
	switch proxyType {
	case ProxyTypeSOCKS5:
		// SOCKS5 relays both TCP and UDP, so the only exclusion is ICMP (no such
		// command) plus the non-network kinds.
		switch kind {
		case "http", "tcp", "nat":
			return true
		case "dns":
			return proxiedDNSHasEndpoint(p)
		}
		return false
	case ProxyTypeHTTP:
		switch kind {
		case "http", "tcp":
			return true
		case "dns":
			// Only the stream-framed resolver protocols ride a CONNECT tunnel. "" (system
			// resolver) and "udp" are datagram DNS.
			switch strings.ToLower(strings.TrimSpace(p.ResolverProtocol)) {
			case "tcp", "dot", "doh":
				return proxiedDNSHasEndpoint(p)
			}
			return false
		case "nat":
			// STUN over a TCP-framed transport tunnels; udp and dtls do not. "" defaults
			// to udp (see ProbeParams.NATTransport).
			switch strings.ToLower(strings.TrimSpace(p.NATTransport)) {
			case "tcp", "tls":
				return true
			}
			return false
		}
		return false
	case ProxyTypeWireGuard:
		// Everything the agent probes is IP traffic, and the tunnel carries IP —
		// including ICMP echoes and UDP — so only the server-side anchors are out.
		switch kind {
		case "icmp", "http", "tcp", "nat":
			return true
		case "dns":
			return proxiedDNSHasEndpoint(p)
		}
		return false
	}
	return false
}

// proxiedDNSHasEndpoint reports whether a DNS monitor names a resolver a proxy could
// actually relay to.
//
// A DNS probe with no ResolverServer uses the SYSTEM resolver — OS-owned ambient
// config with no address on the wire — so there is nothing for a proxy or tunnel to
// carry. Left capable, such a monitor would resolve straight off the host and report
// SUCCESS while the pinned egress was down, which is the precise fail-open the pin
// exists to prevent. It applies to every proxy type, including WireGuard: a tunnel can
// route packets, but it cannot route "whatever the OS resolver decides to do".
func proxiedDNSHasEndpoint(p ProbeParams) bool {
	return strings.TrimSpace(p.ResolverServer) != ""
}

// AnyProxyCapable reports whether a kind/params combination can use at least one
// proxy type. The console uses it to decide whether to show the proxy picker at
// all, so a gateway or host monitor never grows a control that could only be left
// empty.
func AnyProxyCapable(kind string, p ProbeParams) bool {
	return ProxyCapable(kind, p, ProxyTypeSOCKS5) ||
		ProxyCapable(kind, p, ProxyTypeHTTP) ||
		ProxyCapable(kind, p, ProxyTypeWireGuard)
}

// KnownProxyType reports whether t is a compiled proxy type.
func KnownProxyType(t string) bool {
	switch t {
	case ProxyTypeSOCKS5, ProxyTypeHTTP, ProxyTypeWireGuard:
		return true
	}
	return false
}
