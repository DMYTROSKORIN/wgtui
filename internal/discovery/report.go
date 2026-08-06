package discovery

// SystemInfo covers SPEC.md §11.1.
type SystemInfo struct {
	Kernel         string // `uname -srm`
	OSID           string // /etc/os-release ID
	OSVersionID    string // /etc/os-release VERSION_ID
	HasSystemctl   bool
	PackageManager string // "apt", "dnf", "" if unrecognized
	CurrentUser    string
	SudoAvailable  bool // whether the supplied sudo credential actually works
}

// InterfaceDetail covers SPEC.md §11.2 for one WireGuard interface name.
type InterfaceDetail struct {
	Active                   bool
	ConfigExists             bool
	SaveConfig               bool
	RuntimeListenPort        int // 0 if unknown (interface not active, or dump line missing)
	ConfigListenPort         int // 0 if unknown (no config file, or ListenPort unset)
	RuntimePeerPublicKeys    []string
	PersistentPeerPublicKeys []string
	Drift                    bool // computed: runtime and persistent peer sets differ
}

// WireGuardInfo covers SPEC.md §11.2.
type WireGuardInfo struct {
	WgPresent        bool
	WgQuickPresent   bool
	WgVersion        string
	ActiveInterfaces []string
	Interfaces       map[string]InterfaceDetail
}

// NetworkInfo covers the subset of SPEC.md §11.3 relevant to a read-only
// inspector; port-conflict and VPN-subnet-overlap checks belong to the
// Stage 3 setup wizard, which is choosing a *new* subnet/port rather than
// inspecting an existing one.
type NetworkInfo struct {
	DefaultOutboundInterface string
	IPv4Forwarding           bool
	IPv6Forwarding           bool
	ListeningUDPPorts        []int
	FirewallBackend          string // "nftables", "iptables", "ufw", "firewalld", "" if none detected
}

// Report is the full read-only discovery result (SPEC.md §11).
type Report struct {
	System    SystemInfo
	WireGuard WireGuardInfo
	Network   NetworkInfo
	// Errors collects non-fatal check failures (e.g. a command that
	// exited non-zero) so the caller can display them without the whole
	// discovery run failing outright.
	Errors []string
}

// State is one of the seven results from SPEC.md §11.4.
type State string

const (
	StateWireGuardMissing       State = "wireguard_missing"
	StateToolsNoInterfaces      State = "tools_no_interfaces"
	StateConfigExistsStopped    State = "config_exists_stopped"
	StateActiveConsistent       State = "active_consistent"
	StateDrift                  State = "drift"
	StateAmbiguous              State = "ambiguous"
	StateInsufficientPrivileges State = "insufficient_privileges"
)
