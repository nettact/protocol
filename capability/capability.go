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
	ProbeNAT     Capability = "probe.nat"
	InventoryARP Capability = "inventory.arp"
	NetIfaceRead Capability = "network.interface.read"
	NetRouteRead Capability = "network.route.read"

	// Host / system monitoring. Advertised only when the agent is started with
	// the corresponding --report-* flag, so the console can tell whether a given
	// agent will serve host metrics or on-demand process/connection snapshots.
	HostStatRead       Capability = "host.stat.read"       // --report-host
	HostProcessRead    Capability = "host.process.read"    // --report-processes
	HostConnectionRead Capability = "host.connection.read" // --report-connections
	// Reserved for P2+: network.interface.restart, router.openwrt.read,
	// wan.lte.status, wan.lte.failover, …
)
