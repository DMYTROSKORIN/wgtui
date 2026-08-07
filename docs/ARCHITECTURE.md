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

**Stage 3 findings (empirically confirmed by connecting to a disposable VM with no prior
`known_hosts` entry - the first time any live test had actually exercised this path,
since every previously-tested target, including nas.paynet.one, was already known):**

- ssh's host-key confirmation banner is multi-line - the "authenticity of host" warning
  and fingerprint on earlier lines, the actual `(yes/no/[fingerprint])?` question on the
  last - but `runInteractive`'s prompt-detection buffer resets on every `\n`, so a
  condition needing text from two different lines could never fire. The branch had been
  silently unreachable since Stage 0. Fixed by matching only the self-contained question
  line (`isHostKeyQuestionLine`); the `HostKey` callback still receives the full banner
  via the separately-tracked full capture buffer, not the now-single-line prompt snapshot.
- Every `ssh` subprocess inherited `SSH_ASKPASS` from the calling environment. Most
  desktop Linux sessions set this, and ssh can divert an interactive prompt - confirmed
  for the host-key question - to that external GUI helper instead of the PTY `wgtui`
  allocated and is reading from; if the helper can't handle the prompt or there's no
  display, the exchange hangs until wgtui's own timeout kills it. Fixed by forcing
  `SSH_ASKPASS_REQUIRE=never` on every ssh invocation (`newSSHCmd`), so prompt handling is
  deterministic regardless of the environment wgtui runs in.

### osadapter

Per-distribution adapters (Ubuntu/Debian/Fedora in MVP) for:

- OS/package manager detection from `/etc/os-release`.
- WireGuard package installation (spec §12).
- Service management via systemd (`wg-quick@<iface>`) or OpenRC (Stage 4,
  `DetectServiceManager` dispatches independently of distro/package manager - untested
  against a real OpenRC target, since no MVP distro uses it; exists so a future
  non-systemd distro doesn't need a new abstraction layer).
- Firewall/NAT backend detection and rule adapters: nftables, iptables, UFW, firewalld
  (spec §13.5). Every mutating call returns a **plan** (exact commands) first; nothing
  executes without explicit confirmation from the caller. `IsPortOpen` (Stage 4) reads
  each backend's current status first, so re-running the setup wizard against an
  already-configured server is a no-op instead of piling up duplicate `nft`/`iptables`
  rules (those two aren't natively idempotent the way `ufw allow`/`firewall-cmd
  --add-port` already are).

Unsupported distributions get a read-only report, never a guessed mutation (spec §29.6).

NAT/forwarding for a new interface is embedded as `PostUp`/`PostDown` iptables rules tied
to the interface's own up/down lifecycle (`osadapter.NewInterfaceSpec.NAT`), the same
pattern the project's own reference server already uses in practice - it works regardless
of which firewall front-end is active and never touches an existing table, so §13.5's "не
сбрасывать таблицы и цепочки" holds by construction. The firewall adapter's own job is
narrower: just open the WireGuard UDP port on whichever backend is detected.

**Stage 3 findings (empirically confirmed against a disposable QEMU VM, never against
nas.paynet.one - SPEC.md §29.13):**

- `wg-quick strip <path>` requires the path's basename to be exactly `<interface-name>.conf`
  - this affects any code that hands it a differently-named file, not just `transaction`'s
    temp files (see that package's section below for the first instance of this).
- Testing this stage's install/setup flow against a target with *no* prior `known_hosts`
  entry - something every previous live test had accidentally never exercised, since
  nas.paynet.one was already known - surfaced two real bugs in `sshtransport`'s host-key
  confirmation path, documented in that section below.

### wgconfig

Parser/writer for `wg-quick`-style `.conf` files that preserves comments, field order, and
unknown directives (spec §15.5), plus:

- A dedicated **redactor** that strips `[Interface] PrivateKey` before any config text
  leaves the remote host or reaches a log (spec §14, §26.1) — this is a mandatory,
  centralized layer, not an ad hoc call site by call site filter (spec §29.11).
- `wg show ... dump` runtime-state parsing.

### transaction

The lock → ephemeral recovery copy → apply → verify → rollback engine (spec §15.2)
is used by every mutating peer/interface operation (add/disable/resume/delete/rotate). Implemented as a single
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

**Stage 4: multi-interface.** `PeerMetadata.Interface` (set at `peer add` time, carried
through disable/enable/revoke/rotate) is what actually makes a multi-interface server
usable from the CLI: `peerops.ResolveInterface`'s "exactly one active interface, or tell
me which" fallback is the *only* thing available otherwise, and a peer-targeted command
can't ask the user an interactive question mid-flow the way a fresh `peer add` can - it
needs to already know. `cli`'s peer commands also gained an `--interface` override for
metadata created before this field existed.

### profilevault

Local encrypted storage for generated client `.conf` profiles and the peer metadata
registry (spec §7.3, §16.1), under the home directory of the user who ran `wgtui`
(currently `~/.wgtui-config/vault.json`; decision, spec §30.3/§30.5). Encrypted at rest with a master password set on first run
(spec §30.4). Directory `0700`, files `0600`, atomic writes via temp file + rename.
Legacy XDG/Linux and macOS vault paths are migrated atomically on first access. Client
profiles remain inside the encrypted vault unless the user explicitly exports one to
their home directory.

### discovery

Read-only inspection (spec §11): system, WireGuard, network. Classifies the result into
one of the seven discovery states (spec §11.4). Never mutates.

### cli / tui

Both are thin front ends over one shared, structured request/response API (spec §29.4) so
behavior cannot drift between the two. Command surface for `cli` is spec §21; screens for
`tui` are spec §20.

The TUI layout has three responsive tiers: a single panel on narrow terminals, a
server-sidebar/content pair on normal terminals, and Servers/Peers/Selected peer panels
on wide terminals. Peer table widths are derived from the actual panel width, and
`peerops.DefaultDNS` is the shared client-profile DNS default used by TUI, CLI and MCP.

**Stage 4: reconciliation.** `peer list` merges the runtime `wg show all dump` result with
the local metadata registry (`reconcilePeers`, pure/network-free): a peer wgtui knows about
shows its real name and `managed_by_wgtui: true`; anything else is "unmanaged" (spec
§16.1's own description of how an unknown peer should render). A peer wgtui disabled -
deliberately absent from both runtime and persistent config - still appears with state
`disabled` rather than vanishing just because it isn't on the wire. `peer import` (spec
§17.7) is the write-side counterpart: derives a public key from an existing local `.conf`'s
own private key and only registers it if that key is actually found on the server, never
fabricating a match.

### MCP server front-end (`internal/mcpserver`)

Spec §3.1.5 already asks for a non-interactive interface "for automation and AI agents."
`internal/mcpserver` is a third thin front end, alongside `cli` and `tui`, exposing the same
`discovery`/`peerops`/`transaction`/`osadapter`/`profilevault`/`sshtransport` layers over MCP
— so an external AI agent can drive `wgtui` the same way a human drives the TUI: same
validation, same plan-before-apply, same explicit-confirmation requirement for every
mutation. It never calls into `cli` or `tui`, and neither of them calls into it. Run it with
`wgtui mcp` (dispatched from `cmd/wgtui/main.go`, the one place that decides among all three
front ends), serving tools over stdio (`mcp.StdioTransport`).

**Why this matters (not just a nice-to-have, per project owner):** without it, an AI agent
that wants to help a user manage WireGuard peers has to shell out to the `cli` and parse
text/JSON output cold, re-deriving `wgtui`'s whole command surface and safety model (plan-
before-apply, the three-question revoke, secret redaction) from scratch every session with
no guardrails of its own. An MCP front end turns `wgtui` into a backend two different
callers share through the same validated layer - a human via `tui`, an agent via MCP - each
getting the same guarantees (structured errors instead of scraped text, the same plan/
confirm contract, no way to bypass it) rather than the agent needing to be independently
trustworthy about respecting them.

**Tools registered** (`internal/mcpserver/server.go`): `list_servers`, `server_inspect`,
`server_diagnose`, `peer_list` are read-only. `peer_add`, `peer_disable`, `peer_enable`,
`peer_revoke`, `peer_rotate` mutate and are gated by a `confirm bool` argument on every
call: `confirm=false` (the default) only computes and returns the same plan a human would
see and changes nothing on the server or in the local vault; `confirm=true` applies it. Each
mutating tool's `Description` explicitly tells the calling agent never to set `confirm=true`
without the user having seen and approved a prior `confirm=false` preview - this stands in
for cli's interactive `Apply? [y/N]` and tui's plan screen, since MCP has no terminal to
render either on.

**Credential and host-key model** differs from cli/tui out of necessity - see
`docs/THREAT_MODEL.md` T10 for the full trade-off. In short: stdin/stdout *are* the MCP
protocol channel, so SSH/sudo passwords are passed per tool call as arguments
(`internal/mcpserver/credentials.go`'s `Credentials`, embedded in every tool that connects)
rather than prompted for interactively, and used only in-memory for that single call. Host
key confirmation always refuses - a server's first connection must be established once from
a real terminal (`wgtui server inspect`, cli or tui) before it can be driven through MCP.
`peer_add`'s result never includes the generated private key or rendered client profile
(unlike cli's `--show`/`--qr`/`--output`); retrieving the actual profile file is left to
`wgtui peer export`, keeping every mcpserver tool response secret-free by default.

**Shared extraction:** the diagnostic checklist (spec §22) and peer-list reconciliation
(spec §16.1) were promoted from `cli`-private helpers into their own packages,
`internal/diagnostics` and `internal/peerview`, once `mcpserver` became a third consumer of
logic expected to keep growing (both packages had already grown once, adding fields like
`Interface`/`Managed`, since Stage 3/4). Small, stable per-front-end helpers (terminal
prompts, vault bootstrap) stay duplicated between `cli`/`tui`/`mcpserver` deliberately - the
line is whether a third copy of *evolving* logic is worth avoiding, not duplication itself.
`internal/peerview` exists as its own package rather than folding into `peerops` because it
needs `profilevault.PeerMetadata`, and `peerops` never imports `profilevault` directly (see
the `peerops` section above).

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
4. On confirmation, `transaction` executes the locked recovery-copy/apply/verify
   sequence via `sshtransport` and `wgconfig`.
5. On success, `profilevault` persists the new local `.conf` and metadata.
6. On an apply failure, `transaction` restores the temporary recovery copy. It reports
   `recovery required` only if that automatic restore also fails (spec §24).

## Platform support (MVP)

- Control client: Linux amd64/arm64, macOS amd64/arm64.
- Managed server: Ubuntu, Debian, Fedora — systemd only. macOS is a client-only platform,
  never a managed server (spec §5.2, confirmed §30.6).

## Status

The architecture above is implemented. The application is in private preview:
all three front ends and the shared mutation, discovery, encrypted-vault and
platform-adapter layers exist. Remaining work is driven by real-server
compatibility testing and user feedback rather than package scaffolding.
