# Architecture

Source of truth for requirements: [`docs/SPEC.md`](SPEC.md). This document explains how
the code is organized to satisfy that spec, and must be kept in sync as design decisions
are made. See [`docs/THREAT_MODEL.md`](THREAT_MODEL.md) for the security analysis of the
operations described here.

## Guiding constraint

The remote server only ever runs standard WireGuard, standard system configuration files,
and SSH. All control logic lives in the local `wgtui` binary. No daemon, no web UI, no
extra listening port is ever installed on the server (spec §6.1).

## Layers

```
                     ┌─────────────┐   ┌─────────────┐
                     │     cli     │   │     tui     │
                     └──────┬──────┘   └──────┬──────┘
                            │  same request/response API │
                            └─────────────┬───────────────┘
                                          │
          ┌─────────────────────────┼─────────────────────────┐
          │                         │                         │
   ┌──────▼──────┐           ┌──────▼──────┐          ┌───────▼───────┐
   │  discovery  │           │   peerops   │          │  profilevault │
   └──────┬──────┘           └──────┬──────┘          └───────────────┘
          │                         │
          │                  ┌──────┴──────┐
          │                  │             │
          │           ┌──────▼──────┐  ┌───▼───────────┐
          │           │ transaction │  │    wgconfig    │
          │           └──────┬──────┘  └───────┬────────┘
          │                  │                 │
          └─────────┬────────┴─────────┬───────┘
                     │                  │
              ┌──────▼──────┐    ┌──────▼──────┐
              │  osadapter  │    │ sshtransport │
              └──────┬──────┘    └──────┬──────┘
                     │                  │
                     └────────┬─────────┘
                               │
                         ┌─────▼─────┐
                         │  domain   │
                         └───────────┘
```

Import direction is strictly downward. `domain` imports nothing from this project.
`cli` and `tui` are the only packages allowed to be "entry points"; neither may run a
shell command or SSH command directly — everything remote goes through `sshtransport`,
called only from `discovery`, `transaction`, and `osadapter`.

### domain

Pure data model: `Server`, `Peer`, peer status enum (spec §9), no I/O. Business rules that
don't need the network (e.g. peer status heuristic, CIDR overlap check) live here so they
are trivially unit-testable.

### sshtransport

Wraps the system `ssh` binary (spec §6.2 — MVP does not implement its own SSH client).
Responsibilities:

- `ssh -G <alias>` resolution to preview host/user/port without leaking secrets (spec §10.1).
- Host key verification: strict by default, fingerprint prompt on first connect, hard stop
  on mismatch (spec §10.2). Never sets `StrictHostKeyChecking=no`.
- Running remote commands over a single ControlMaster connection.
- Interactive password and `sudo` handling through a PTY, with the password held only in
  process memory for the session lifetime (spec §10.3).
- No secret ever appears as a process argument or in an environment variable.

**Stage 0 finding (empirically confirmed against the project's test server):** spec §10.3's
"`sudo -v` once, then `sudo -n` within the current session" does not hold across separate
`ssh` invocations. `sudo`'s per-tty credential cache (`tty_tickets`, the default on modern
distros) is keyed to the exact pty device the validating `sudo -v` ran on; a later `ssh`
call made without `-tt` allocates no remote pty at all and misses that cache key, so
`sudo -n` is refused again seconds later even though `sudo -v` just succeeded. `RunSudo`
therefore re-authenticates through its own `-tt` channel on every privileged call instead
of depending on a cross-process cache; `SudoValidate` remains useful only as an early
fail-fast check that the supplied password is correct. See `internal/sshtransport/client.go`
doc comments for the mechanism.

### osadapter

Per-distribution adapters (Ubuntu/Debian/Fedora in MVP) for:

- OS/package manager detection from `/etc/os-release`.
- WireGuard package installation (spec §12).
- Service management via systemd (`wg-quick@<iface>`).
- Firewall/NAT backend detection and rule adapters: nftables, iptables, UFW, firewalld
  (spec §13.5). Every mutating call returns a **plan** (exact commands) first; nothing
  executes without explicit confirmation from the caller.

Unsupported distributions get a read-only report, never a guessed mutation (spec §29.6).

### wgconfig

Parser/writer for `wg-quick`-style `.conf` files that preserves comments, field order, and
unknown directives (spec §15.5), plus:

- A dedicated **redactor** that strips `[Interface] PrivateKey` before any config text
  leaves the remote host or reaches a log (spec §14, §26.1) — this is a mandatory,
  centralized layer, not an ad hoc call site by call site filter (spec §29.11).
- `wg show ... dump` runtime-state parsing.

### transaction

The backup → lock → apply → verify → rollback engine (spec §15.2) used by every mutating
peer/interface operation (add/disable/resume/delete/rotate). Implemented as a single
remote script per mutation (built by `internal/transaction`, run via
`sshtransport.Client.RunSudoScript`) rather than several separate SSH round trips: one
sudo prompt, one atomic pass, and no window between our own separate commands for the
remote state to move out from under us. Locking uses an atomic `mkdir` lock directory
(spec §15.3's documented fallback, used unconditionally here since a `flock` held by one
`ssh` process can't be relied on to still be held by a *different* process for a later
step anyway — see sshtransport's `RunSudo` doc comment for the same reasoning applied to
sudo's own cache). Live apply is `wg syncconf` fed by `wg-quick strip`, so unrelated peers
are not disconnected.

**Stage 2 findings (empirically confirmed against the project's test server):**

- Driving a command through a local PTY (`-tt`, needed for the sudo password prompt)
  introduces two byte-level artifacts that don't affect line-oriented parsing but do
  break anything that needs an *exact* copy of remote content: cooked-mode ONLCR
  translation turns every `\n` into `\r\n`, and a stray "space + CRLF" line lands at the
  very start of the real output right after a successful password prompt (matches sudo's
  own line-clearing behavior). `transaction.Fingerprint` hashes the config exactly as
  read, so an uncorrected capture never matched the server's own `sha256sum` - the TOCTOU
  guard failed on *every* run, not just a real concurrent edit. Fixed by
  `sshtransport.normalizePTYCapture`, applied to `RunSudo`/`RunSudoScript` output.
- `wg-quick strip <path>` derives the interface name from the path's basename and
  requires it to be exactly `<name>.conf` - a temp file named `wg0.conf.wgtui.tmp.$$`
  fails that check outright even though its content is valid. Fixed by writing the temp
  file into a same-filesystem subdirectory (`.wgtui-tmp/`) under the real name
  (`$IFACE.conf`), which also needs no per-process suffix since the mkdir lock already
  serializes access to one transaction per interface at a time.

### peerops

Orchestrates one full peer lifecycle operation (add/disable/resume/revoke/rotate, spec
§17) by composing `wgconfig` (parse the current config, build/locate a peer block),
`transaction` (apply it), and local key generation — this is the layer `cli` and `tui`
both call so the logic isn't duplicated between them. It never touches `profilevault`
directly: `DisablePeer` returns the removed block's exact bytes and `AddPeer`/`RotatePeer`
return the generated keys, and it's the caller's job to decide what to persist and where
(SPEC.md §7.1's save/QR/clipboard/stdout choices are a `cli`/`tui` concern, not `peerops`'s).

### profilevault

Local encrypted storage for generated client `.conf` profiles and the peer metadata
registry (spec §7.3, §16.1), under the home directory of the user who ran `wgtui`
(decision, spec §30.3/§30.5). Encrypted at rest with a master password set on first run
(spec §30.4). Directory `0700`, files `0600`, atomic writes via temp file + rename.

### discovery

Read-only inspection (spec §11): system, WireGuard, network. Classifies the result into
one of the seven discovery states (spec §11.4). Never mutates.

### cli / tui

Both are thin front ends over one shared, structured request/response API (spec §29.4) so
behavior cannot drift between the two. Command surface for `cli` is spec §21; screens for
`tui` are spec §20.

### Future extension: MCP server front-end (post-MVP, not started)

Spec §3.1.5 already asks for a non-interactive interface "for automation and AI agents."
A natural post-MVP addition is a third thin front end, `internal/mcpserver`, exposing the
same request/response API as `cli`/`tui` over MCP — so an external AI agent can drive
`wgtui` the same way a human drives the TUI: same validation, same plan-before-apply, same
explicit-confirmation requirement for every mutation. It changes nothing about the trust
model, because it is still the human confirming a shown plan.

**Explicitly rejected:** `wgtui` calling out to an external AI service to make a decision
*on behalf of* the user for an ambiguous or unresolved situation (e.g. an install choice,
a conflicting discovery state, whether to proceed after drift). That would be a hidden
cloud control plane (excluded by spec §3.2), would send server/network details to a third
party, opens a prompt-injection path through attacker-influenced remote content (see
`docs/THREAT_MODEL.md` T8), and replaces the explicit human confirmation that every
mutating operation currently depends on for safety (§13.5, §20.4). If this is ever
reconsidered, it must stay strictly read-only/advisory and never be allowed to approve a
mutation by itself.

## Data flow for a mutating operation (e.g. peer add)

1. `cli`/`tui` builds a request from user input.
2. `transaction` package validates it against current `domain` state pulled fresh via
   `discovery`/`wgconfig`.
3. A **plan** (diff, no secrets) is produced and shown to the user (spec §20.4); nothing
   is sent to the server yet.
4. On confirmation, `transaction` executes the locked backup/apply/verify sequence via
   `sshtransport` and `wgconfig`.
5. On success, `profilevault` persists the new local `.conf` and metadata.
6. On any failure at any step, `transaction` restores the backup and reports
   `recovery required` rather than guessing (spec §24).

## Platform support (MVP)

- Control client: Linux amd64/arm64, macOS amd64/arm64.
- Managed server: Ubuntu, Debian, Fedora — systemd only. macOS is a client-only platform,
  never a managed server (spec §5.2, confirmed §30.6).

## Status

This document describes the target shape. As of this commit only the package skeleton
(`doc.go` stubs) and `cmd/wgtui` exist. Implementation starts at Stage 0 (see the project
task board / `docs/SPEC.md` §28).
