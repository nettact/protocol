package telemetry

// HealthLayer tags which layer of the network stack a metric or event belongs
// to, per the layered diagnosis model in architecture §4. The rule engine uses
// these to localize a fault (local vs LAN vs WAN vs DNS vs service).
type HealthLayer string

const (
	LayerLocal    HealthLayer = "local"    // 本机层: NIC, IP, route, proxy, VPN
	LayerLAN      HealthLayer = "lan"      // 局域网层: gateway, ARP, DHCP, Wi-Fi
	LayerWAN      HealthLayer = "wan"      // WAN 层: ISP gateway, public IP, PPPoE
	LayerInternet HealthLayer = "internet" // 互联网层: multiple public targets
	LayerDNS      HealthLayer = "dns"      // DNS 层: system/public/DoH resolvers
	LayerService  HealthLayer = "service"  // 服务层: specific sites, APIs, VPN
	LayerWireless HealthLayer = "wireless" // 无线层: RSSI, channel, band, retries
)
