package peerops

import (
	"encoding/binary"
	"fmt"
	"net/netip"
)

// NextFreeAddress returns the lowest IPv4 address within cidr that is
// neither the network address, the broadcast address, nor marked used -
// SPEC.md §17.2's "автоматический... VPN-адрес" plus the uniqueness and
// subnet-membership checks it asks for, satisfied by construction rather
// than validated after the fact.
func NextFreeAddress(cidr netip.Prefix, used map[netip.Addr]bool) (netip.Addr, error) {
	if !cidr.IsValid() || !cidr.Addr().Is4() {
		return netip.Addr{}, fmt.Errorf("no usable IPv4 VPN subnet known for this interface")
	}
	network := cidr.Masked().Addr()
	broadcast := lastAddrV4(cidr)

	addr := network.Next()
	for addr.IsValid() && addr.Compare(broadcast) < 0 {
		if !used[addr] {
			return addr, nil
		}
		addr = addr.Next()
	}
	return netip.Addr{}, fmt.Errorf("no free address available in %s", cidr)
}

// lastAddrV4 computes the broadcast (highest) address of an IPv4 prefix.
func lastAddrV4(p netip.Prefix) netip.Addr {
	a4 := p.Masked().Addr().As4()
	ip := binary.BigEndian.Uint32(a4[:])
	hostBits := uint(32 - p.Bits())
	var mask uint32
	if hostBits >= 32 {
		mask = 0xFFFFFFFF
	} else {
		mask = (uint32(1) << hostBits) - 1
	}
	last := ip | mask
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], last)
	return netip.AddrFrom4(b)
}
