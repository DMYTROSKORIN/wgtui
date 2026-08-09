# wgtui — project guide for AI agents

You're reading this because you're about to work in this repository. This
file is the orientation doc: what the project is, how it's built, what the
non-negotiable rules are, and where to look for more depth. Read
[`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md) next - it's the running
checkpoint of exactly what state the project is in right now, updated at
the end of every work session.

## What this is

`wgtui` is a terminal UI, CLI, and MCP server for managing remote WireGuard
servers over plain SSH - no agent on the server, no web UI, no mesh
platform. See [`README.md`](README.md) for the full pitch and
[`docs/SPEC.md`](docs/SPEC.md) for the complete behavioral specification
every feature is built against (numbered sections; code comments cite it as
`SPEC.md §N.M` - when in doubt about intended behavior, that document is the
source of truth, not this one).

## Architecture, in one page

Three front ends share one validated core - see
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full map:

- `internal/tui` — the Bubble Tea terminal UI (`cmd/wgtui`, no args).
- `internal/cli` — the scriptable command surface (`wgtui server ...`,
  `wgtui peer ...`).
- `internal/mcpserver` — the MCP stdio server for AI agents (`wgtui mcp`).

All three call into the same lower layers and never call each other:

- `internal/discovery` — read-only inspection of a server's WireGuard state.
- `internal/osadapter` — distro-specific install/configure adapters
  (Ubuntu/Debian/Fedora).
- `internal/peerops` — peer key generation, allocation, config assembly.
- `internal/transaction` — the locked-copy / verify / rollback-on-failure
  primitive every remote config write goes through.
- `internal/profilevault` — the encrypted local registry of servers and
  client peer profiles.
- `internal/sshtransport` — wraps the system `ssh` binary over a PTY, since
  interactive password/host-key prompts have to be seen and answered.
- `internal/wgconfig` — parses/writes/redacts WireGuard config text.
- `internal/diagnostics`, `internal/peerview` — shared read-model logic used
  by more than one front end.

## Non-negotiable rules

These come directly from `docs/SPEC.md` and `docs/THREAT_MODEL.md` and have
been load-bearing since before this file existed. Don't relax them for
convenience:

- **Never start a mutating remote action without showing the exact plan and
  getting explicit confirmation first** - install, configure, every peer
  lifecycle action. This applies identically in the TUI, CLI (`--yes` still
  requires the flag to be typed), and MCP (`confirm=true` still requires
  the caller to have shown a `confirm=false` preview first).
- **No secret ever appears in an argument, an environment variable read by
  someone other than `wgtui` itself, a log line, or `--json` output.** SSH
  and sudo passwords go straight to a PTY's stdin, never through argv.
  `WGTUI_MASTER_PASSWORD` is the one deliberate, documented exception (MCP
  has no terminal to prompt on) - see README's MCP section for the
  trade-off that requires.
- **Client private keys are generated locally and never sent to the
  server.** The server only ever receives a public key.
- **Every remote config write is a locked-copy / verify / rollback-on-
  failure transaction** (`internal/transaction`), never a direct overwrite.
- **Strict SSH host-key verification, always** - no code path may set
  `StrictHostKeyChecking=no`.
- **`wgtui` refuses to run as root.**

## Building, testing, releasing

```bash
go build ./...
go vet ./...
go test ./...
go test -race ./...
gofmt -l .                       # must be empty
~/go/bin/staticcheck ./...       # must be empty
~/go/bin/govulncheck ./...       # 0 called vulnerabilities
scripts/semgrep-scan.sh          # 0 findings; also runs as a pre-push hook
```

All of the above must pass before a release. Releases are cut by pushing an
annotated tag:

```bash
git tag -a vX.Y.Z -m "..."
git push origin vX.Y.Z
```

`.github/workflows/release.yml` (triggered by that tag push) runs Semgrep,
the full validation suite including Darwin cross-builds, then GoReleaser,
which publishes `.tar.gz`/`.deb`/`.rpm` archives to a GitHub Release. Update
the running install with `scripts/install.sh vX.Y.Z` afterward, and confirm
with `wgtui --version` / `sha256sum ~/.local/bin/wgtui` against the
published `checksums.txt`. Update `CHANGELOG.md` and
`docs/PROJECT_STATUS.md` as part of the same release - the latter is what
the *next* session (human or agent) reads first to know what's already
done.

## Testing philosophy specific to this project

`go test ./...` is necessary but not sufficient for TUI changes. Several
real bugs in this project's history were invisible to unit tests and only
showed up rendering in an actual terminal:

- A renderer indenting a multi-line block by prepending spaces to the whole
  string (only the first line actually moves) instead of indenting every
  line.
- ANSI escape codes getting corrupted by a helper meant for plain text
  (stripping the ESC control byte out of a color code leaves the rest of
  the escape sequence as visible garbage).
- `bubbles/table`'s internal scroll state producing visually different
  behavior than what a unit test checking `Cursor()`/`View()` in isolation
  would suggest.

When changing rendering, mouse hit-testing, or scroll/geometry logic:
verify live in a real terminal, not just via `go test`. This project's
convention: build the binary, launch it in an isolated `tmux` session with
`HOME` pointed at a scratch directory (so it creates a fresh vault, never
touching the real one), and drive it with `tmux send-keys` /
`tmux capture-pane -p` (add `-e` to inspect raw ANSI when checking for
color/escape-code bugs specifically). Raw mouse clicks can be sent as SGR
mouse sequences via `tmux send-keys -H <hex bytes>`
(`\x1b[<0;COL;ROWM` for a left-button press, 1-indexed coordinates).

Whenever you add a renderer or a mouse hit-test that must agree on
geometry, share the actual computation (a constant or a small function)
between the two - never let the renderer and the hit-test each hardcode the
same numbers independently. This project has been bitten by that drifting
more than once; grep for `commanderSegmentWidths`, `detailTabsRow`,
`peerTableHeaderRow` for the established pattern.

There's a live disposable test VM used for install/discovery/credential
flow verification: `192.168.122.245` (skorin / see prior session context
or ask the user for the current password - it changes). Treat it as
genuinely disposable but don't repeat a full install/configure pass
casually; check `docs/PROJECT_STATUS.md` for what state it's already in.

## Commit and release-note conventions

- Commit messages: imperative mood, explain *why* not just *what*, no
  `Co-Authored-By` line.
- `CHANGELOG.md` follows Keep a Changelog style, one version section per
  release.
- `docs/PROJECT_STATUS.md` is a running journal, most recent release cycle
  first. Each entry should let a future session (or a future you) resume
  cold: what was reported, what the actual root cause was (not just the
  symptom), what test proves it's fixed, and release evidence (commit,
  tag, run ID, installed SHA-256).

## Where the rest of the documentation lives

| Doc | What it's for |
| --- | --- |
| [`README.md`](README.md) | Public-facing pitch, install, usage |
| [`docs/SPEC.md`](docs/SPEC.md) | The full behavioral specification (source of truth) |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Package map and the reasoning behind it |
| [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md) | Assets, trust boundaries, threats and mitigations |
| [`docs/TUI.md`](docs/TUI.md) | Terminal UI behavior contract |
| [`docs/COMMANDER_UI.md`](docs/COMMANDER_UI.md) | Top-menu/bottom-bar accelerator contract specifically |
| [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md) | Current state, release history, resume point |
| [`CHANGELOG.md`](CHANGELOG.md) | User-facing per-release changes |

If something in this file and `docs/SPEC.md` ever disagree, `docs/SPEC.md`
wins - update this file to match, not the other way around.
