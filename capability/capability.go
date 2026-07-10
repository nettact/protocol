// Package capability enumerates the abilities an agent advertises at
// enrollment (architecture §3.4). The server must not assume every agent can
// do everything: an agent's capability set is the intersection of the
// collectors it runs and what the host platform actually supports.
package capability

type Capability string

const (
	ProbeICMP    Capability = "probe.icmp"
	ProbeDNS     Capability = "probe.dns"
	ProbeHTTP    Capability = "probe.http"
	ProbeTCP     Capability = "probe.tcp"
	InventoryARP Capability = "inventory.arp"
	NetIfaceRead Capability = "network.interface.read"
	NetRouteRead Capability = "network.route.read"
	// Reserved for P2+: network.interface.restart, router.openwrt.read,
	// wan.lte.status, wan.lte.failover, …
)
