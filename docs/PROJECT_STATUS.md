# Project checkpoint

Last updated: 2026-08-09

Active branch: `master`

Release target: next release after `v0.4.0` (unversioned as of this checkpoint)

Release status: **v0.4.0 published, installed and complete; the top-menu/bottom-bar
redesign from `docs/UI_MENU_REDESIGN_HANDOFF.md` is implemented, tested and merged
into `master`, but not yet tagged or released**

This file is the durable restart point for work on `wgtui`. After an interrupted
session, read it before changing code, then confirm the actual repository state
with `git status --short --branch` and `git log -5 --oneline`.

## Completed since `v0.4.0`

- Implemented `docs/UI_MENU_REDESIGN_HANDOFF.md` in full: the top menu
  (`Alt+letter`) and the bottom action bar (plain letter) are now two
  explicitly separate accelerator systems that never promise the same key
  twice. Removed the `│` separators from the top row; its active/dark
  background now shows only while a dropdown is actually open, never merely
  because a related screen is showing. Bottom-bar accelerator letters are
  highlighted inside their own label (`Remove`, `revOke`) instead of a
  separate key cell; `Remove` moved from `X` to `R` and `Revoke` from `X` to
  `O` so both are letters that actually appear in their labels. Added a
  shared `renderAcceleratedLabel` renderer and shared width-geometry helpers
  so the mouse hit-test can never disagree with what was rendered.
- Added acceptance tests for the redesign contract (`internal/tui/menu_redesign_test.go`):
  no `│` at any width, active background only while the matching dropdown is
  open, every non-symbol accelerator letter is present inside its own label,
  per-panel accelerator uniqueness, and mouse click / keyboard accelerator
  dispatching the same action.
- Verified the new chrome live in a tmux session at `64×18` and `100×30`
  against an isolated `HOME`, including inspecting raw ANSI output to confirm
  the active category actually renders with the dark background and calm
  categories render with the cyan one.
- Ran the full missing-WireGuard install flow for real (not read-only) against
  the disposable VM `192.168.122.245` (Ubuntu 24.04) through the actual TUI:
  discovery correctly classified `wireguard_missing`, the confirmation screen
  showed the exact `apt-get update` / `apt-get install -y wireguard` plan,
  applying it after the sudo-password prompt succeeded, `VerifyInstalled`
  passed, and the server reclassified to `tools_no_interfaces` with the
  bottom bar switching from `Install` to `Configure`. Independently confirmed
  over a second SSH session that `wireguard`/`wireguard-tools` are installed
  via `dpkg -l` and `wg --version`. The VM now has WireGuard installed but no
  interface configured; configuring an interface was out of this task's scope.
- `go build`, `go vet`, `staticcheck v0.7.0`, `govulncheck v1.6.0` (0 called
  vulnerabilities), `go test ./...` and `go test -race ./...`,
  `CGO_ENABLED=0` Darwin amd64/arm64 cross-builds, and `goreleaser check` all
  pass on the changed tree.

## Completed in the `v0.4.0` release cycle

- Reviewed `Screenshot_20260808_180214.png` and the matching Midnight
  Commander screenshot. Confirmed that the top and bottom chrome needed solid
  contrast, explicit separators, a compact dropdown and real mouse hit areas.
- Replaced the visible F-key strip with contextual btop-style highlighted
  letter accelerators. F-keys remain undocumented compatibility aliases only.
- Enabled Bubble Tea cell-motion mouse support and added click dispatch for top
  categories, dropdown items and bottom actions; disabled actions remain safe.
- Moved initial vault create/unlock into a branded in-app password form.
- Replaced terminal release/read-password flows with in-app SSH host-key, SSH
  password and sudo password modals. Secrets are cleared from model fields and
  cached only inside the active remote session.
- Added access, credential, mouse, disabled-click, exact-width and minimum-size
  regressions.
- Anchored the final action row to the terminal's physical bottom row. Status
  banners no longer shift the visible bar away from its mouse hit area.
- Letter accelerators now honor the same readiness rules as disabled menu and
  mouse actions; they cannot bypass missing WireGuard/interface prerequisites.
- Verified the disposable VM `192.168.122.245` is Ubuntu 24.04 with systemd
  and no `wg` binary. The new read-only manual integration test passed full
  discovery, sudo validation, missing-WireGuard classification, diagnostics,
  distro detection and install-plan generation without mutating the server.

- Published and locally installed `v0.2.2`; the release contains Darwin and
  Linux archives for amd64/arm64 plus Linux `.deb`/`.rpm` packages.
- Installed `v0.2.2` binary SHA-256:
  `31a31434fd01f8ef91b1d4c9b3573dd66321457f42285b0da8ac56c456acbf8a`.
- User approved a pure-Go Commander UI direction inspired by Midnight
  Commander; S-Lang/CGO migration is explicitly out of scope to preserve macOS
  compatibility and the tested Go business layer.
- Added `docs/COMMANDER_UI.md` as the menu, action-bar, two-pane and cross-platform
  behavior contract for `v0.3.0`.
- Implemented the persistent Server/Peer/View/Tools/Help menu with disabled
  reasons, mouse/letter navigation and safe dispatch through existing workflows.
- The former F-key strip was superseded in `v0.4.0` by a contextual segmented
  letter-action bar whose rendered width is exact at every supported width from
  64 through 300 columns.
- Overview, Peers and Diagnose now retain stable collection/detail panes in
  wide mode; compact peer actions fit the supported `64×18` minimum.
- CI and release validation now cross-build both Darwin architectures with
  `CGO_ENABLED=0` before packaging.
- PR #13 passed CI #28 and merged into `master` as `4828162`.
- Annotated tag `v0.3.0` points at `4828162`; release workflow #6 completed
  Semgrep, validation and GoReleaser successfully.
- GitHub Release `v0.3.0` is published with checksums, four Linux/macOS
  archives, two `.deb` files and two `.rpm` files.
- `scripts/install.sh v0.3.0` verified the published Linux amd64 archive and
  installed the static `wgtui 0.3.0` binary at `~/.local/bin/wgtui`.
- Installed `v0.3.0` binary SHA-256:
  `bf0d425bbe4779e4ebaf0e3d30fb9c66a68431af3a860d8301442daf35476f52`.

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

### Current `v0.4.0` branch

- `go test ./...`, `go test -race ./...`, `go vet ./...` and Staticcheck
  `v0.7.0`: pass.
- govulncheck `v1.6.0`: 0 called vulnerabilities and 0 vulnerabilities in
  imported packages.
- Semgrep: 101 rules on 112 repository files, 0 findings; a second explicit
  scan covered the four new security-relevant untracked Go files with 0
  findings.
- Darwin amd64 and arm64 `CGO_ENABLED=0` builds: valid Mach-O executables.
- Two 10-second fuzz runs: about 1.27M terminal-safety executions and 1.51M
  export-path executions, no failures.
- GoReleaser configuration check and clean snapshot: pass; Linux/macOS
  amd64/arm64 archives plus Linux deb/rpm packages and checksums produced.
- Read-only missing-WireGuard Ubuntu 24.04 VM integration: pass; no remote
  mutation was applied.
- PR #15 CI run `31269116803`: pass after stabilizing the vault guidance across
  runner-dependent text widths. Linux/macOS builds, vet, unit, race,
  Staticcheck, govulncheck and Semgrep are green at commit `98f3e0c`.
- PR #15 merged into `master` as `a63d3fb`; master CI run `31269480947`
  passed all build, macOS cross-build, vet, unit, race, Staticcheck,
  govulncheck and Semgrep gates.
- Annotated tag `v0.4.0` points at `a63d3fb` and is pushed to origin.
- Release workflow `31269636963` passed Semgrep, validation and GoReleaser.
- GitHub Release `v0.4.0` is published (not draft/prerelease) with checksums,
  four Linux/macOS archives, two `.deb` files and two `.rpm` files.
- `scripts/install.sh v0.4.0` verified the published Linux amd64 archive and
  installed `wgtui 0.4.0` at `~/.local/bin/wgtui` with mode `0755`.
- Installed `v0.4.0` binary SHA-256:
  `4a3f284fdf44955518a046a0ea3441726f2daa3827f53d795b82ed457d5556a3`.

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

The menu redesign from `docs/UI_MENU_REDESIGN_HANDOFF.md` is implemented,
tested and pushed to `master`, but no version bump, tag or GitHub release has
been cut for it yet - that is the next step if a release is wanted. Do not
repeat the already-passed live install test against `192.168.122.245` unless
install/discovery behavior changes; that VM currently has WireGuard installed
with no interface configured.

## Publishing note

`git push` works over the configured SSH remote. GitHub CLI is authenticated and
the connected GitHub app works for PR and merge operations.

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
