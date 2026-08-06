package osadapter

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

var interfaceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,15}$`)

// NewInterfaceSpec describes the [Interface] section for a brand-new
// wg-quick config. Every field is validated by CreateInterface before it
// can reach the remote script.
type NewInterfaceSpec struct {
	Name       string // e.g. "wg0"
	Address    netip.Prefix
	Address6   netip.Prefix // zero value (IsValid() == false) if none
	ListenPort int
	// NAT, if true, embeds PostUp/PostDown iptables rules for full-tunnel
	// masquerading through OutboundInterface - this is the same pattern
	// the project's own reference server uses (see docs/ARCHITECTURE.md),
	// and it works regardless of which firewall front-end (ufw,
	// firewalld, bare nftables/iptables) manages the rest of the system,
	// since it's tied to the interface's own up/down lifecycle rather
	// than a separate persistent firewall rule set (SPEC.md §13.5's "не
	// сбрасывать таблицы и цепочки" is satisfied by construction: this
	// never touches an existing table at all).
	NAT               bool
	OutboundInterface string
}

func (s NewInterfaceSpec) validate() error {
	if !interfaceNamePattern.MatchString(s.Name) {
		return fmt.Errorf("invalid interface name %q", s.Name)
	}
	if !s.Address.IsValid() {
		return fmt.Errorf("a valid VPN IPv4 address/CIDR is required")
	}
	if s.ListenPort < 1 || s.ListenPort > 65535 {
		return fmt.Errorf("listen port %d out of range", s.ListenPort)
	}
	if s.NAT && !interfaceNamePattern.MatchString(s.OutboundInterface) {
		return fmt.Errorf("invalid outbound interface name %q", s.OutboundInterface)
	}
	return nil
}

// CreateInterface generates a fresh WireGuard keypair *on the server* via
// `wg genkey`, never letting the private key leave the shell variable it's
// generated into or appear as a command-line argument (SPEC.md §13.3):
// it's written directly into the config file server-side and the
// function returns only the public key. Refuses to overwrite an existing
// config at the target path.
func CreateInterface(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, spec NewInterfaceSpec) (publicKey string, err error) {
	if err := spec.validate(); err != nil {
		return "", err
	}

	script := buildCreateInterfaceScript(spec)
	res, err := client.RunSudoScript(ctx, script, prompts)
	if err != nil {
		return "", fmt.Errorf("run interface creation script: %w", err)
	}
	out := string(res.Stdout)

	if strings.Contains(out, "WGTUI_ALREADY_EXISTS") {
		return "", fmt.Errorf("/etc/wireguard/%s.conf already exists - refusing to overwrite", spec.Name)
	}
	idx := strings.Index(out, "WGTUI_PUBKEY")
	if idx == -1 || res.ExitCode != 0 {
		return "", fmt.Errorf("interface creation script did not report success (exit %d): %s", res.ExitCode, out)
	}
	pub := strings.TrimSpace(strings.TrimPrefix(out[idx:], "WGTUI_PUBKEY"))
	if fields := strings.Fields(pub); len(fields) > 0 {
		pub = fields[0]
	}
	return pub, nil
}

func buildCreateInterfaceScript(spec NewInterfaceSpec) string {
	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "CONF=%s\n", shellQuote("/etc/wireguard/"+spec.Name+".conf"))
	b.WriteString(`if [ -e "$CONF" ]; then echo WGTUI_ALREADY_EXISTS; exit 1; fi` + "\n")
	b.WriteString("umask 077\n")
	b.WriteString(`mkdir -p "$(dirname "$CONF")"` + "\n")
	b.WriteString(`PRIVKEY=$(wg genkey)` + "\n")
	b.WriteString(`PUBKEY=$(printf '%s' "$PRIVKEY" | wg pubkey)` + "\n")

	b.WriteString("{\n")
	b.WriteString(`  echo "[Interface]"` + "\n")
	b.WriteString(`  echo "PrivateKey = $PRIVKEY"` + "\n")
	addr := spec.Address.String()
	if spec.Address6.IsValid() {
		addr += ", " + spec.Address6.String()
	}
	fmt.Fprintf(&b, "  echo %s\n", shellQuote("Address = "+addr))
	fmt.Fprintf(&b, "  echo %s\n", shellQuote("ListenPort = "+strconv.Itoa(spec.ListenPort)))
	if spec.NAT {
		postUp := fmt.Sprintf("PostUp = iptables -A FORWARD -i %%i -j ACCEPT; iptables -A FORWARD -o %%i -j ACCEPT; iptables -t nat -A POSTROUTING -o %s -j MASQUERADE", spec.OutboundInterface)
		postDown := fmt.Sprintf("PostDown = iptables -D FORWARD -i %%i -j ACCEPT; iptables -D FORWARD -o %%i -j ACCEPT; iptables -t nat -D POSTROUTING -o %s -j MASQUERADE", spec.OutboundInterface)
		fmt.Fprintf(&b, "  echo %s\n", shellQuote(postUp))
		fmt.Fprintf(&b, "  echo %s\n", shellQuote(postDown))
	}
	b.WriteString(`} > "$CONF"` + "\n")
	b.WriteString(`chmod 600 "$CONF"` + "\n")
	b.WriteString(`echo "WGTUI_PUBKEY $PUBKEY"` + "\n")

	return b.String()
}

// shellQuote wraps s in POSIX single quotes, escaping any embedded single
// quote - the standard technique for making a string inert to shell
// interpretation. Duplicated from internal/transaction's identical helper
// deliberately: each package that builds a remote script quotes
// independently rather than sharing a cross-package helper for ~3 lines.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
