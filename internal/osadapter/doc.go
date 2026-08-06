// Package osadapter provides per-distribution adapters for package
// installation, service management (systemd, later OpenRC) and firewall
// backends (nftables, iptables, UFW, firewalld). Every mutating adapter
// method returns a plan describing exact commands before it is allowed to
// execute anything.
package osadapter
