package sshtransport

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
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

// newSSHCmd builds an exec.Cmd for the ssh binary with SSH_ASKPASS_REQUIRE
// forced to "never". Without this, ssh honors an inherited SSH_ASKPASS
// (set by essentially every desktop Linux session, e.g. KDE's
// ksshaskpass) and can divert a prompt to that external GUI helper
// instead of writing it to the PTY we allocated and are reading from -
// confirmed while testing against a disposable VM: a first-time host-key
// confirmation silently went to a GUI dialog neither we nor the
// interactive user watching the terminal could answer, hanging until our
// own timeout killed it. This had never been exercised before because
// every previously-tested target was already a known host, so Dial()
// never hit the host-key-prompt branch live. Forcing "never" here makes
// prompt handling deterministic regardless of the environment wgtui runs
// in.
func newSSHCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Env = append(os.Environ(), "SSH_ASKPASS_REQUIRE=never")
	return cmd
}

func (c *Client) baseArgs() []string {
	args := []string{
		// Keep genuine connection errors visible while suppressing routine
		// multiplex/PTY chatter such as "Shared connection ... closed".
		// `-q` hid both, leaving callers with an unactionable empty exit 255.
		"-o", "LogLevel=ERROR",
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
	_ = newSSHCmd(ctx, args...).Run()
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
	cmd := newSSHCmd(ctx, args...)
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
	res := interactiveResult(out, err)
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return res, err
}

// RunSudoScript runs an arbitrary multi-line bash script as root, used by
// internal/transaction to send its whole backup/write/validate/apply
// sequence as one atomic remote round trip instead of several separate
// RunSudo calls (fewer round trips, smaller window for concurrent state
// change, and only one sudo password prompt).
//
// The script is base64-encoded for transport - `echo <b64> | base64 -d |
// sudo bash` - so its own internal quoting (a caller typically embeds a
// config file's content via a heredoc) never has to interact with the
// outer command-line quoting: the outer shell only ever sees base64
// alphabet characters. `bash`, not `sh`, because the transaction script
// uses `<(...)` process substitution for `wg syncconf`, which dash (the
// default /bin/sh on Debian/Ubuntu) does not support.
func (c *Client) RunSudoScript(ctx context.Context, script string, prompts Prompts) (Result, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	remoteCmd := "echo " + encoded + " | base64 -d | sudo bash"
	args := append(c.baseArgs(), "-tt", c.target.HostOrAlias, remoteCmd)
	out, err := c.runInteractive(ctx, args, prompts)
	res := interactiveResult(out, err)
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return res, err
}

// interactiveResult separates successful command output from diagnostics.
// A PTY is a single combined stream, so failed ssh/sudo commands have no
// distinct stderr pipe. Keeping their capture in Stderr prevents callers from
// displaying an empty "exited 255:" message and preserves the actual SSH or
// sudo explanation.
func interactiveResult(out []byte, err error) Result {
	normalized := normalizePTYCapture(out)
	if err != nil {
		return Result{Stderr: normalized}
	}
	return Result{Stdout: normalized}
}

// normalizePTYCapture undoes two artifacts introduced by driving a
// command through a local PTY rather than a plain pipe (confirmed against
// nas.paynet.one:2626 by comparing a captured `cat` of a config file
// against `sha256sum` run on the same file directly: the two never
// matched until this was added):
//
//  1. The pty's line discipline runs in cooked mode by default, which
//     applies ONLCR translation (outgoing "\n" becomes "\r\n") - so
//     captured multi-line output has different bytes than the real file,
//     even though it reads identically.
//  2. After a successful sudo password prompt, one stray "space + CRLF"
//     line lands at the very start of the real command's output - this
//     matches sudo's own end-of-password-entry line-clearing behavior,
//     not anything the command itself printed.
//
// This matters because RunSudo/RunSudoScript are used to fetch config
// text whose exact bytes get hashed (transaction.Fingerprint) and later
// reproduced verbatim in a new config - a corrupted capture would never
// match the server's own sha256sum, and SPEC.md §15.2's TOCTOU check
// would fail every single time, not just on an actual concurrent edit.
func normalizePTYCapture(out []byte) []byte {
	s := strings.TrimLeft(string(out), " \t\r\n")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return []byte(s)
}

// runInteractive spawns ssh under a local PTY, scans its output for a
// handful of known ssh/sudo prompts, and answers them via the Prompts
// callbacks. It returns everything read after the last prompt it answered
// (i.e. with prompt boilerplate stripped from the front) — for a plain
// Dial/SudoValidate call (no real command output expected) callers ignore
// this; RunSudo returns it as the command's captured stdout.
func (c *Client) runInteractive(ctx context.Context, args []string, prompts Prompts) ([]byte, error) {
	cmd := newSSHCmd(ctx, args...)
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
					return []byte(full.String()[outputStart:]), werr
				default:
					continue
				}
			}
			// EOF (or another read error) from the pty: the remote side
			// almost certainly closed because the process exited.
			werr := <-done
			return []byte(full.String()[outputStart:]), werr
		}
		lineBuf.WriteByte(b)
		full.WriteByte(b)
		text := lineBuf.String()
		if prompts.Debug != nil {
			prompts.Debug(text)
		}

		switch {
		case isHostKeyQuestionLine(text):
			// ssh's host-key confirmation is a multi-line banner: "The
			// authenticity of host '...' can't be established." and the
			// fingerprint are on earlier lines, but this loop's lineBuf
			// resets on every '\n' (see the b == '\n' case below), so by
			// the time the actual "(yes/no/[fingerprint])?" question line
			// arrives, lineBuf no longer contains those earlier lines -
			// matching on "authenticity of host" here (as this used to)
			// could never be true at the same time as the trailing "?",
			// so this branch was silently unreachable in real use.
			// Confirmed by running against a disposable VM freshly
			// generating a first-time host key: every previous live
			// target had happened to already be in known_hosts, so this
			// path had never actually executed until then. Matching only
			// the self-contained final question line - which the pattern
			// above does - fixes the trigger; the fingerprint the user
			// needs to see is recovered separately below from the full
			// captured buffer, since lineBuf itself no longer has it.
			fullPrompt := full.String()[outputStart:]
			ok := false
			if prompts.HostKey != nil {
				ok = prompts.HostKey(fullPrompt)
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

// isHostKeyQuestionLine matches ssh's host-key confirmation question -
// deliberately just the self-contained final line ("Are you sure you want
// to continue connecting (yes/no/[fingerprint])? "), not anything from the
// earlier "The authenticity of host ... can't be established." / fingerprint
// lines of that banner, since lineBuf (this function's only caller passes
// its current contents) resets on every '\n' and would never contain text
// from two different lines at once.
func isHostKeyQuestionLine(text string) bool {
	return strings.Contains(text, "Are you sure you want to continue connecting") &&
		strings.HasSuffix(strings.TrimRight(text, " "), "?")
}

func isTimeoutErr(err error) bool {
	type timeout interface{ Timeout() bool }
	te, ok := err.(timeout)
	return ok && te.Timeout()
}
