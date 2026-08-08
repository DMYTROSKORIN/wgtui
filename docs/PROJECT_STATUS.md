# Project checkpoint

Last updated: 2026-08-08 12:25 CEST

Active branch: `feat/commander-ui-v0.3.0`

Release target: `v0.3.0`

Release status: **Commander UI complete; v0.3.0 release cycle in progress**

This file is the durable restart point for work on `wgtui`. After an interrupted
session, read it before changing code, then confirm the actual repository state
with `git status --short --branch` and `git log -5 --oneline`.

## Completed in the current release cycle

- Published and locally installed `v0.2.2`; the release contains Darwin and
  Linux archives for amd64/arm64 plus Linux `.deb`/`.rpm` packages.
- Installed `v0.2.2` binary SHA-256:
  `31a31434fd01f8ef91b1d4c9b3573dd66321457f42285b0da8ac56c456acbf8a`.
- User approved a pure-Go Commander UI direction inspired by Midnight
  Commander; S-Lang/CGO migration is explicitly out of scope to preserve macOS
  compatibility and the tested Go business layer.
- Added `docs/COMMANDER_UI.md` as the menu, F-key, two-pane and cross-platform
  behavior contract for `v0.3.0`.
- Implemented the persistent Server/Peer/View/Tools/Help menu with disabled
  reasons, F9 navigation and safe action dispatch through existing workflows.
- Implemented a contextual ten-slot F1-F10 bar whose rendered width is exact at
  every supported width from 64 through 300 columns.
- Overview, Peers and Diagnose now retain stable collection/detail panes in
  wide mode; compact peer actions fit the supported `64×18` minimum.
- CI and release validation now cross-build both Darwin architectures with
  `CGO_ENABLED=0` before packaging.

- Reviewed the new `3063×1335` screenshot and reproduced its effective
  wide-layout sizes in the text renderer.
- Removed nested ANSI formatting from peer table cells so the parent table can
  render one continuous selected-row background and repaint safely.
- Added a semantic peer summary and the running version to the application
  header, making both state and screenshot provenance immediately visible.
- Added a wide-layout invariant that checks fixed positions for all four panel
  borders at every width `135..260`.

- Fixed stale asynchronous discovery/peer results crossing server or refresh
  boundaries by attaching server and request generation IDs.
- Peers opened during initial discovery wait for the session and load
  automatically instead of racing a missing connection.
- SSH exit 255 now retains diagnostics and triggers one bounded automatic
  reconnect; a second failure becomes a stable, actionable error.
- Fixed the reported broken cyan panel lines: table calculations now include
  renderer cell padding, so rows cannot overwrite the panel border.
- Long remote error messages are sanitized, wrapped and bounded inside the
  peer panel.

- Replaced the ambiguous post-peer-creation screen with an explicit client
  setup menu: QR, private profile+QR export, secret config text, or finish.
- Added four layout modes: below-minimum guard, compact, standard, and wide.
  Minimum is `64×18`; recommended size is `90×24`.
- Wide mode now requires adequate height as well as width. Short wide terminals
  fall back to the compact single-panel layout instead of overflowing.
- Shrinking a terminal or crossing a breakpoint requests a full renderer clear,
  preventing duplicated footers and stale borders.
- Added bounded diagnostic scrolling, deterministic interface ordering,
  many-interface summaries, QR height safety, compact help/overview screens,
  tiny-terminal fallback, and ANSI-safe status/footer truncation.
- Applied semantic colors across navigation, selection, connection/success,
  unmanaged/attention, error/danger, values, and secondary context. Every color
  retains a text label or symbol.
- Strengthened CI and release workflows with race tests, Staticcheck and
  govulncheck. Added `CHANGELOG.md` to release archives.
- Updated README, TUI and architecture documentation for the new contract.

## Test and packaging evidence

- `go test ./...`: pass.
- `go test -race ./...`: pass.
- `go vet ./...`: pass.
- Staticcheck `v0.7.0`: pass after fixing all findings.
- govulncheck `v1.6.0`: 0 called/package vulnerabilities. It reports only the
  unmaintained `x/crypto/openpgp` package at module level; wgtui does not import
  or call it.
- Semgrep: 101 rules on 108 tracked files, 0 findings locally, in the
  pre-push hook, and in PR CI.
- Fuzzing: about 305k terminal-text/truncation inputs and 1.25M export-name
  inputs, no failures.
- Responsive property tests cover every width `64..180` over eight terminal
  heights, plus tiny sizes below the supported minimum.
- GoReleaser `v2.17.1` configuration check: pass.
- GoReleaser snapshot: pass for Linux/macOS amd64/arm64, producing four archives,
  four Linux packages and valid checksums. Archives contain the binary, license,
  README, changelog, man page and three shell completions.
- actionlint `v1.7.12`: pass for CI and release workflows.
- New peer-loading and rendering regressions pass, including every wide
  terminal width `135..260` and the reported screenshot-equivalent width.
- The affected TUI and SSH transport packages passed 20 consecutive test runs.
- Two additional 10-second fuzz runs completed about 390k terminal-text and
  662k export-path executions without failure.
- Commander UI verification passed: all package and race tests, vet,
  Staticcheck, Semgrep, govulncheck (0 called vulnerabilities), Darwin
  amd64/arm64 cross-builds, GoReleaser check/snapshot, about 596k terminal-text
  fuzz executions and 806k export-path fuzz executions.
- PR #10 passed build, vet, unit, race, Staticcheck, govulncheck and Semgrep CI,
  then merged into `master` as `0665b82`.
- Master CI #23 passed.
- Annotated tag `v0.2.1` points at `0665b82` and is pushed to origin.
- Release workflow #4 passed Semgrep, validation and GoReleaser.
- GitHub Release `v0.2.1` is published (not draft/prerelease) with checksums,
  four Linux/macOS archives, two `.deb` files and two `.rpm` files.
- `scripts/install.sh v0.2.1` verified the published archive checksum and
  installed `wgtui 0.2.1` at `~/.local/bin/wgtui` with mode `0755`.
- Installed `v0.2.1` binary SHA-256:
  `8f04b163e3ba25277fd8d8e354ad863116c9fcd55f6958f942292bca6ac48e2d`.
- PR #7 (`Release v0.2.0: polished responsive TUI`) passed both GitHub CI
  jobs, including build, vet, unit, race, Staticcheck, govulncheck and Semgrep.
- PR #7 was merged into `master` as `682924c`.
- Final checkpoint PR #8 passed CI and was merged as `f337526`.
- Annotated tag `v0.2.0` points at `f337526` and is pushed to origin.
- Master CI #19 passed.
- Release workflow #3 passed Semgrep, build, vet, race, Staticcheck,
  govulncheck and GoReleaser.
- GitHub Release `v0.2.0` is published (not draft/prerelease) with checksums,
  four Linux/macOS archives, two `.deb` files and two `.rpm` files.
- `scripts/install.sh v0.2.0` verified the published archive checksum and
  installed `wgtui 0.2.0` at `~/.local/bin/wgtui` with mode `0755`.
- Installed binary SHA-256:
  `ecb9853a17ed28beebca3f10d67df8c8b11d3d1be1467b77cb8426026bb2e61a`.

## Resume here

The `v0.3.0` implementation and local release checks are complete. Resume by
committing/pushing the implementation, opening its PR, waiting for CI, merging,
tagging `v0.3.0`, verifying the release assets and installing the published
binary locally.

## Publishing note

`git push` works over the configured SSH remote. The connected GitHub app works
for PR, CI and merge operations even though the local `gh` API token is invalid.

## Useful commands

```bash
cd /home/skorin/Git/wgtui
git status --short --branch
git log -5 --oneline
go test ./...
go test -race ./...
go vet ./...
~/.local/bin/goreleaser check
```
