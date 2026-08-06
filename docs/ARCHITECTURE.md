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
                 ┌────────────────────────┼────────────────────────┐
                 │                        │                        │
          ┌──────▼──────┐         ┌───────▼───────┐        ┌───────▼───────┐
          │  discovery  │         │  transaction  │        │  profilevault │
          └──────┬──────┘         └───────┬───────┘        └───────────────┘
                 │                        │
        ┌────────┴────────┐      ┌────────┴────────┐
        │                 │      │                 │
 ┌──────▼──────┐   ┌──────▼──────▼──────┐   ┌───────▼──────┐
 │  osadapter  │   │      wgconfig      │   │ sshtransport │
 └──────┬──────┘   └─────────────────────┘   └───────┬──────┘
        │                                             │
        └───────────────────┬─────────────────────────┘
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
- Running remote commands and short scripts over a single session/ControlMaster.
- Interactive password and `sudo -v` / `sudo -n` handling through a PTY, with the password
  held only in process memory for the session lifetime (spec §10.3).
- No secret ever appears as a process argument or in an environment variable.

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
peer/interface operation (add/disable/resume/delete/rotate). Locking uses `flock` where
available, falling back to atomic lock-directory creation (spec §15.3). Preferred live
apply is `wg syncconf` fed by `wg-quick strip`, so unrelated peers are not disconnected.

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
