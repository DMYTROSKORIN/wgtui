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
// can be seen and answered exactly once. Subsequent RunCommand/RunSudo
// calls reuse this authenticated connection and must not prompt at all.
func (c *Client) Dial(ctx context.Context, prompts Prompts) error {
	args := append(c.baseArgs(), "-tt", c.target.HostOrAlias, "true")
	return c.runInteractive(ctx, args, prompts)
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
// once (SPEC.md §10.3). Subsequent privileged commands should use RunSudo,
// which relies on the timestamp cache this call establishes.
func (c *Client) SudoValidate(ctx context.Context, prompts Prompts) error {
	args := append(c.baseArgs(), "-tt", c.target.HostOrAlias, "sudo -v")
	return c.runInteractive(ctx, args, prompts)
}

// RunSudo runs remoteCmd with `sudo -n`, relying on the timestamp cache
// established by a prior SudoValidate call. It never carries the sudo
// password itself, and never falls back to prompting silently.
func (c *Client) RunSudo(ctx context.Context, remoteCmd string) (Result, error) {
	return c.RunCommand(ctx, "sudo -n "+remoteCmd)
}

// runInteractive spawns ssh under a local PTY, scans its output for a
// handful of known ssh/sudo prompts, and answers them via the Prompts
// callbacks. It is intentionally simple (byte-at-a-time scanning of a
// short-lived line buffer) since Stage 0 only needs to get past login and
// sudo prompts, not drive a general-purpose remote shell.
func (c *Client) runInteractive(ctx context.Context, args []string, prompts Prompts) error {
	cmd := exec.CommandContext(ctx, "ssh", args...)
	f, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start pty: %w", err)
	}
	defer f.Close()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	reader := bufio.NewReader(f)
	var lineBuf strings.Builder
	deadline := time.Now().Add(60 * time.Second)

	for {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			return fmt.Errorf("timed out waiting for an ssh prompt or completion")
		}
		_ = f.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		b, rerr := reader.ReadByte()
		if rerr != nil {
			if isTimeoutErr(rerr) {
				select {
				case werr := <-done:
					return classifyExit(werr)
				default:
					continue
				}
			}
			// EOF (or another read error) from the pty: the remote side
			// almost certainly closed because the process exited.
			werr := <-done
			return classifyExit(werr)
		}
		lineBuf.WriteByte(b)
		text := lineBuf.String()

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
				return fmt.Errorf("host key not accepted (no HostKey callback, or callback declined)")
			}
			lineBuf.Reset()
		case strings.HasSuffix(text, "assword: ") || strings.HasSuffix(text, "assword:"):
			// Matches both ssh's "user@host's password:" and sudo's
			// "[sudo] password for user:".
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
				return fmt.Errorf("answering password prompt: %w", perr)
			}
			io.WriteString(f, pw+"\n")
			lineBuf.Reset()
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
