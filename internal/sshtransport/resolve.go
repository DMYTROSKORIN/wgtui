package sshtransport

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// Resolved is the effective host/port/user ssh would use for a Target,
// obtained via `ssh -G`, without opening a connection or touching any
// secret (SPEC.md §10.1: "показать вычисленные host/user/port, не
// раскрывая секреты").
type Resolved struct {
	Hostname     string
	Port         int
	User         string
	IdentityFile string
}

// ResolveAlias runs `ssh -G` for t and parses the result. It never prompts
// and never connects.
func ResolveAlias(ctx context.Context, t Target) (Resolved, error) {
	args := resolveArgs(t)
	cmd := exec.CommandContext(ctx, "ssh", args...)
	out, err := cmd.Output()
	if err != nil {
		return Resolved{}, fmt.Errorf("ssh -G %s: %w", t.HostOrAlias, err)
	}
	return parseResolved(strings.NewReader(string(out)))
}

func resolveArgs(t Target) []string {
	args := []string{"-G"}
	if t.Port != 0 {
		args = append(args, "-p", strconv.Itoa(t.Port))
	}
	if t.User != "" {
		args = append(args, "-l", t.User)
	}
	if t.IdentityFile != "" {
		args = append(args, "-i", t.IdentityFile)
	}
	return append(args, t.HostOrAlias)
}

// parseResolved parses `ssh -G` output. Split out from ResolveAlias so it
// can be unit tested against captured sample output without invoking the
// real ssh binary.
func parseResolved(r io.Reader) (Resolved, error) {
	var res Resolved
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, val, ok := strings.Cut(sc.Text(), " ")
		if !ok {
			continue
		}
		switch key {
		case "hostname":
			res.Hostname = val
		case "port":
			if p, err := strconv.Atoi(val); err == nil {
				res.Port = p
			}
		case "user":
			res.User = val
		case "identityfile":
			if res.IdentityFile == "" { // ssh -G may repeat this line; keep the first
				res.IdentityFile = val
			}
		}
	}
	return res, sc.Err()
}
