package transaction

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// validInterfaceName mirrors internal/discovery's check. Duplicated
// deliberately: this package is the one embedding the value into a shell
// script, so it validates independently rather than trusting an upstream
// caller to have already done so (SPEC.md §23.1 - validate at the point
// where a value is about to be used to build a command, not just
// somewhere upstream).
var validInterfaceName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,15}$`)

// buildScript renders Plan into the bash script sshtransport.Client.
// RunSudoScript will run as root on the target. It implements SPEC.md
// §15.2's full sequence in one remote round trip:
//
//  1. acquire an interface-scoped lock (atomic mkdir, §15.3)
//  2. re-check the config's fingerprint against what Apply's caller last
//     read (TOCTOU guard, step 3)
//  3. timestamped temporary recovery copy (step 4)
//  4. write the new content to a temp file on the same filesystem (step 5)
//  5. preserve the original owner/mode (step 6)
//  6. validate with `wg-quick strip` before it's live (step 7)
//  7. atomically rename into place (step 8)
//  8. apply with `wg syncconf`, which - fed a stripped config - doesn't
//     disconnect peers that didn't change (step 9)
//  9. on apply failure, restore the recovery copy and try to re-sync it (step 11)
//  10. remove the temporary rollback copy after success or a verified rollback;
//     retain it only when automatic recovery itself fails
//  11. release the lock via `trap ... EXIT` regardless of outcome (step 12)
//
// The new config content is embedded via a heredoc with a random
// delimiter so it never needs shell-escaping itself; RunSudoScript then
// base64-transports the whole script so *that* heredoc's quoting never
// interacts with the outer ssh command line either.
func buildScript(plan Plan) (string, error) {
	if plan.Interface == "" || plan.ConfigPath == "" || plan.ExpectedFingerprint == "" {
		return "", fmt.Errorf("incomplete plan: interface, config path, and expected fingerprint are all required")
	}
	if !validInterfaceName.MatchString(plan.Interface) {
		return "", fmt.Errorf("refusing to build a transaction script for an unsafe interface name %q", plan.Interface)
	}

	delim, err := randomDelimiter()
	if err != nil {
		return "", err
	}
	if strings.Contains(plan.NewContent, delim) {
		// Astronomically unlikely with a random 128-bit token, but check
		// rather than silently truncate the payload at a false heredoc
		// terminator if it ever did happen.
		return "", fmt.Errorf("internal error: generated delimiter collided with config content, try again")
	}

	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "IFACE=%s\n", shellSingleQuote(plan.Interface))
	fmt.Fprintf(&b, "CONF=%s\n", shellSingleQuote(plan.ConfigPath))
	b.WriteString(`LOCKDIR="/run/lock/wgtui-$IFACE.lock"` + "\n")
	b.WriteString(`if ! mkdir "$LOCKDIR" 2>/dev/null; then echo WGTUI_LOCK_FAILED; exit 1; fi` + "\n")
	b.WriteString(`BACKUP=""; TMP=""; TMPDIR=""; KEEP_BACKUP=0` + "\n")
	b.WriteString(`cleanup() { rc=$?; rm -f "$TMP" /tmp/wgtui-verr.$$ /tmp/wgtui-aerr.$$ 2>/dev/null || true; ` +
		`[ -z "$TMPDIR" ] || rmdir "$TMPDIR" 2>/dev/null || true; ` +
		`if [ "$KEEP_BACKUP" -eq 0 ]; then [ -z "$BACKUP" ] || rm -f "$BACKUP" || true; ` +
		`else echo "WGTUI_RECOVERY_REQUIRED $BACKUP"; fi; ` +
		`rmdir "$LOCKDIR" 2>/dev/null || true; return "$rc"; }` + "\n")
	b.WriteString(`trap cleanup EXIT` + "\n")

	b.WriteString(`CURRENT_SUM=$(sha256sum "$CONF" 2>/dev/null | cut -d' ' -f1)` + "\n")
	fmt.Fprintf(&b, "if [ \"$CURRENT_SUM\" != %s ]; then echo WGTUI_FINGERPRINT_MISMATCH; exit 1; fi\n", shellSingleQuote(plan.ExpectedFingerprint))

	b.WriteString(`OWNER=$(stat -c '%u:%g' "$CONF")` + "\n")
	b.WriteString(`MODE=$(stat -c '%a' "$CONF")` + "\n")
	b.WriteString(`TS=$(date +%Y%m%d%H%M%S)` + "\n")
	b.WriteString(`BACKUP="$CONF.bak.$TS"` + "\n")
	b.WriteString(`cp -p "$CONF" "$BACKUP"` + "\n")

	// wg-quick derives the interface name from its argument's basename
	// and requires it to be exactly "<name>.conf" (name matching its own
	// IFNAMSIZ-shaped pattern) - "$CONF.wgtui.tmp.$$" fails that check
	// even though its *content* would be perfectly valid, confirmed
	// against nas.paynet.one:2626 ("wg-quick: The config file must be a
	// valid interface name, followed by .conf"). A same-filesystem
	// subdirectory lets the temp file keep the real "$IFACE.conf" name
	// for wg-quick's benefit while never colliding with the real path.
	// No per-process suffix is needed: the mkdir lock above already
	// guarantees only one wgtui transaction runs against this interface
	// at a time.
	b.WriteString(`TMPDIR="$(dirname "$CONF")/.wgtui-tmp"` + "\n")
	b.WriteString(`mkdir -p "$TMPDIR"` + "\n")
	b.WriteString(`TMP="$TMPDIR/$IFACE.conf"` + "\n")
	fmt.Fprintf(&b, "cat > \"$TMP\" <<'%s'\n", delim)
	b.WriteString(plan.NewContent)
	if !strings.HasSuffix(plan.NewContent, "\n") {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%s\n", delim)

	b.WriteString(`chown "$OWNER" "$TMP"` + "\n")
	b.WriteString(`chmod "$MODE" "$TMP"` + "\n")

	b.WriteString(`if ! wg-quick strip "$TMP" > /dev/null 2>/tmp/wgtui-verr.$$; then ` +
		`echo WGTUI_VALIDATE_FAILED; cat /tmp/wgtui-verr.$$ 1>&2; rm -f /tmp/wgtui-verr.$$ "$TMP"; exit 1; fi` + "\n")
	b.WriteString(`rm -f /tmp/wgtui-verr.$$` + "\n")

	b.WriteString(`KEEP_BACKUP=1` + "\n")
	b.WriteString(`mv "$TMP" "$CONF"` + "\n")

	b.WriteString(`if wg syncconf "$IFACE" <(wg-quick strip "$CONF") 2>/tmp/wgtui-aerr.$$; then ` +
		`KEEP_BACKUP=0; echo WGTUI_OK; ` +
		`else cp -p "$BACKUP" "$CONF"; ` +
		`if wg syncconf "$IFACE" <(wg-quick strip "$CONF") 2>>/tmp/wgtui-aerr.$$; then ` +
		`KEEP_BACKUP=0; echo WGTUI_APPLY_FAILED_ROLLED_BACK; fi; ` +
		`cat /tmp/wgtui-aerr.$$ 1>&2; rm -f /tmp/wgtui-aerr.$$; exit 1; fi` + "\n")

	return b.String(), nil
}

// shellSingleQuote wraps s in POSIX single quotes, escaping any embedded
// single quote as '\” - the standard technique for making a string
// completely inert to shell interpretation (nothing is special inside
// single quotes except a literal single quote itself).
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func randomDelimiter() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate heredoc delimiter: %w", err)
	}
	return "WGTUI_PAYLOAD_" + hex.EncodeToString(b), nil
}
