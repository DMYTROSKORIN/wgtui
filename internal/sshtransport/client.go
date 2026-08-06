package sshtransport

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
)

// Prompts lets a caller answer interactive prompts coming from the system
// ssh/sudo binaries. Every answer is written straight to the PTY's stdin —
// never to a process argument or environment variable (SPEC.md §10.3,
// §23.1: password must not appear in argv, env, or a debug log).
type Prompts struct {
	// HostKey is called with the exact text ssh printed (including the
	// fingerprint) when a host is not yet in known_hosts, or has changed.
	// Returning false aborts the dial. A nil HostKey rejects by default —
	// callers must opt in to accepting new host keys.
	HostKey func(prompt string) bool
	// Password supplies the SSH login password when key/agent auth is not
	// available or is refused.
	Password func() (string, error)
	// SudoPassword supplies the password for SudoValidate's `sudo -v`.
	SudoPassword func() (string, error)
	// Debug, if set, receives every accumulated (non-secret) output chunk
	// seen while scanning for a prompt. It never receives anything typed
	// back (passwords), only what the remote side sent - useful for
	// diagnosing a prompt-matching miss without risking a secret leak.
	Debug func(chunk string)
}

// Client is one SSH ControlMaster connection to a Target, multiplexed so
// that repeated commands don't each pay a fresh handshake/auth cost.
type Client struct {
	target      Target
	controlPath string
}

// NewClient does not connect. controlPath must live in a directory only
// this user can read (0700) — typically under $XDG_RUNTIME_DIR/wgtui or an
// equivalent private temp directory — since anyone able to reach the
// socket can run commands through this authenticated session.
func NewClient(target Target, controlPath string) *Client {
	return &Client{target: target, controlPath: controlPath}
}

func (c *Client) baseArgs() []string {
	args := []string{
		"-q", // suppress ssh's own client-side diagnostics (e.g. "Shared
		// connection to <host> closed."), which would otherwise land in
		// the same pty stream as real remote command output when a
		// command is run through a multiplexed (-tt) channel.
		"-o", "ControlMaster=auto",
		"-o", "ControlPersist=600",
		"-o", "ControlPath=" + c.controlPath,
		"-o", "StrictHostKeyChecking=ask",
		"-o", "BatchMode=no",
	}
	if c.target.Port != 0 {
		args = append(args, "-p", strconv.Itoa(c.target.Port))
	}
	if c.target.User != "" {
		args = append(args, "-l", c.target.User)
	}
	if c.target.IdentityFile != "" {
		args = append(args, "-i", c.target.IdentityFile)
	}
	return args
}

// Dial establishes the ControlMaster connection by running a trivial
// remote command ("true") through a PTY, so a host-key or password prompt
// can be seen and answered exactly once. Subsequent RunCommand calls reuse
// this authenticated connection and must not prompt at all.
func (c *Client) Dial(ctx context.Context, prompts Prompts) error {
	args := append(c.baseArgs(), "-tt", c.target.HostOrAlias, "true")
	_, err := c.runInteractive(ctx, args, prompts)
	return err
}

// Close tears down the ControlMaster connection. A missing/already-closed
// master is not treated as an error worth surfacing.
func (c *Client) Close(ctx context.Context) {
	args := append(c.baseArgs(), "-O", "exit", c.target.HostOrAlias)
	_ = exec.CommandContext(ctx, "ssh", args...).Run()
}

// Result is the outcome of a non-interactive remote command.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// RunCommand runs remoteCmd non-interactively over the already-authenticated
// ControlMaster connection. BatchMode=yes makes ssh fail fast instead of
// hanging if it would unexpectedly need to prompt.
func (c *Client) RunCommand(ctx context.Context, remoteCmd string) (Result, error) {
	args := append(c.baseArgs(), "-o", "BatchMode=yes", c.target.HostOrAlias, remoteCmd)
	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := Result{Stdout: []byte(stdout.String()), Stderr: []byte(stderr.String())}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("ssh run %q: %w", remoteCmd, err)
	}
	return res, nil
}

// SudoValidate runs an interactive `sudo -v` over a fresh PTY channel so
// the remote "[sudo] password for <user>:" prompt can be seen and answered
// (SPEC.md §10.3), as an early, cheap check that the supplied sudo
// password is actually correct before starting a longer flow.
//
// It intentionally does NOT make later RunSudo calls password-free: sudo's
// per-tty credential cache (the default "tty_tickets" behavior on modern
// distros) is keyed to the specific pty device the validating `sudo -v`
// ran on. A later ssh invocation without -tt allocates no remote tty at
// all, misses that cache key, and is asked for a password again — this
// was confirmed empirically against the project's test server (see
// docs/ARCHITECTURE.md, Stage 0 notes). RunSudo therefore re-authenticates
// on every call via its own -tt channel instead of depending on this
// cache; the SPEC.md §10.3 "sudo -n within the current session" reuse
// window did not hold up across separate ssh processes on this target.
func (c *Client) SudoValidate(ctx context.Context, prompts Prompts) error {
	args := append(c.baseArgs(), "-tt", c.target.HostOrAlias, "sudo -v")
	_, err := c.runInteractive(ctx, args, prompts)
	return err
}

// RunSudo runs remoteCmd under sudo, answering the "[sudo] password for
// <user>:" prompt itself via a dedicated PTY channel every time (see
// SudoValidate's doc comment for why it cannot rely on a cache). The
// password is held only in prompts.SudoPassword's closure and is written
// straight to the PTY, never to argv or the environment (SPEC.md §23.1).
func (c *Client) RunSudo(ctx context.Context, remoteCmd string, prompts Prompts) (Result, error) {
	args := append(c.baseArgs(), "-tt", c.target.HostOrAlias, "sudo "+remoteCmd)
	out, err := c.runInteractive(ctx, args, prompts)
	res := Result{Stdout: out}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return res, err
}

// runInteractive spawns ssh under a local PTY, scans its output for a
// handful of known ssh/sudo prompts, and answers them via the Prompts
// callbacks. It returns everything read after the last prompt it answered
// (i.e. with prompt boilerplate stripped from the front) — for a plain
// Dial/SudoValidate call (no real command output expected) callers ignore
// this; RunSudo returns it as the command's captured stdout.
func (c *Client) runInteractive(ctx context.Context, args []string, prompts Prompts) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ssh", args...)
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}
	defer f.Close()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	reader := bufio.NewReader(f)
	var lineBuf strings.Builder
	var full strings.Builder
	outputStart := 0
	deadline := time.Now().Add(60 * time.Second)

	for {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("timed out waiting for an ssh prompt or completion")
		}
		_ = f.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		b, rerr := reader.ReadByte()
		if rerr != nil {
			if isTimeoutErr(rerr) {
				select {
				case werr := <-done:
					return []byte(full.String()[outputStart:]), classifyExit(werr)
				default:
					continue
				}
			}
			// EOF (or another read error) from the pty: the remote side
			// almost certainly closed because the process exited.
			werr := <-done
			return []byte(full.String()[outputStart:]), classifyExit(werr)
		}
		lineBuf.WriteByte(b)
		full.WriteByte(b)
		text := lineBuf.String()
		if prompts.Debug != nil {
			prompts.Debug(text)
		}

		switch {
		case strings.Contains(text, "authenticity of host") && strings.Contains(text, "?"):
			ok := false
			if prompts.HostKey != nil {
				ok = prompts.HostKey(text)
			}
			if ok {
				io.WriteString(f, "yes\n")
			} else {
				io.WriteString(f, "no\n")
				_ = cmd.Process.Kill()
				return nil, fmt.Errorf("host key not accepted (no HostKey callback, or callback declined)")
			}
			lineBuf.Reset()
			outputStart = full.Len()
		case strings.Contains(strings.ToLower(text), "password") && (strings.HasSuffix(text, ": ") || strings.HasSuffix(text, ":")):
			// Matches both ssh's "user@host's password: " and sudo's
			// "[sudo] password for user: " - "password" is not
			// necessarily at the very end of the line (sudo appends
			// "for <user>: " after it), so we match on containment plus
			// a trailing colon, not a fixed suffix.
			var pw string
			var perr error
			if strings.Contains(text, "sudo") {
				if prompts.SudoPassword != nil {
					pw, perr = prompts.SudoPassword()
				} else {
					perr = fmt.Errorf("sudo asked for a password but no SudoPassword callback was provided")
				}
			} else if prompts.Password != nil {
				pw, perr = prompts.Password()
			} else {
				perr = fmt.Errorf("ssh asked for a password but no Password callback was provided")
			}
			if perr != nil {
				_ = cmd.Process.Kill()
				return nil, fmt.Errorf("answering password prompt: %w", perr)
			}
			io.WriteString(f, pw+"\n")
			lineBuf.Reset()
			outputStart = full.Len()
		case b == '\n':
			lineBuf.Reset()
		}
	}
}

func isTimeoutErr(err error) bool {
	type timeout interface{ Timeout() bool }
	te, ok := err.(timeout)
	return ok && te.Timeout()
}

func classifyExit(err error) error {
	if err == nil {
		return nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("ssh exited with %s", exitErr.String())
	}
	return err
}
