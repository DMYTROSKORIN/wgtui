// Package domain contains the core data model: Server, Peer, their statuses
// and the pure business rules that do not depend on SSH, the filesystem, or
// any UI. Nothing in this package may import sshtransport, osadapter,
// transaction, profilevault, cli or tui.
package domain
