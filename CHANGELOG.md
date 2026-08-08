# Changelog

All notable release changes are documented here. Versions follow semantic
versioning while the project remains below 1.0.

## [Unreleased]

### Changed

- Top menu and bottom action bar are now two clearly separate accelerator
  systems, per `docs/UI_MENU_REDESIGN_HANDOFF.md`: `Alt+letter` always opens
  a top-menu category regardless of screen, while a plain letter always runs
  a bottom-bar action for the current screen only - the same letter can mean
  different things on different screens without conflict (e.g. plain `P`
  changes the vault password on the server list, `Alt+P` opens the Peer menu
  everywhere).
- Removed the `│` separators from the top menu row; adjacent categories are
  told apart by even padding alone.
- The top menu's active (dark) background now shows only while its dropdown
  is actually open, never just because a related screen or tab happens to be
  showing.
- Bottom-bar accelerator letters are now highlighted inside their own label
  text (`Remove`, `revOke`) instead of being printed as a separate key cell
  next to the label (`X Remove`). `Remove` moved from `X` to `R` and `Revoke`
  from `X` to `O` so both accelerators are letters that actually appear in
  their labels.
- Added a single ANSI/Unicode-safe `renderAcceleratedLabel` renderer shared
  by both the top menu and the bottom bar, and shared width-geometry helpers
  so mouse hit-testing can never disagree with what was rendered.
- The keyboard help screen now explains the `Alt+letter` vs plain-letter
  contract explicitly, with per-screen bottom-bar tables.

## [0.4.0] - 2026-08-08

### Added

- Mouse-clickable top categories, dropdown actions and segmented bottom actions.
- Branded in-app forms for local master-password unlock, SSH host-key trust,
  SSH login passwords and remote sudo passwords.
- Highlighted letter accelerators throughout the menu and action bar, following
  btop's discoverable shortcut convention.

### Changed

- Replaced the visually continuous F1-F10 strip with sharply separated,
  contextual letter actions.
- Remote work no longer releases Bubble Tea's alternate screen to read a
  password from the plain terminal.

## [0.3.0] - 2026-08-08

### Added

- A persistent Commander-style Server / Peer / View / Tools / Help menu,
  reachable with F9, including visible explanations for unavailable actions.
- A contextual ten-slot F1–F10 action bar and keyboard help covering the full
  primary workflow without requiring memorized single-letter shortcuts.
- Stable wide two-pane layouts for Overview, Peers and Diagnose, with
  recommended actions or selected-object details in the right pane.

### Changed

- Destructive F8/menu actions always enter an explicit confirmation or peer
  action screen; they never apply a change immediately.
- Compact peer actions omit secondary metadata to remain usable at `64×18`.

### Verification

- Added menu navigation, reachability, disabled-state, safe dispatch, session
  cleanup, exact F-bar width and two-pane layout regressions.
- CI and release validation now cross-build `CGO_ENABLED=0` macOS clients for
  both amd64 and arm64 before packaging.

## [0.2.2] - 2026-08-08

### Changed

- The TUI header shows the running version, making screenshots and reports
  distinguish an old process from a newly installed binary.
- The peer dashboard adds a compact semantic summary for total, online,
  attention and unmanaged peers.
- Peer selection now uses one uninterrupted full-row highlight while semantic
  detail colors remain in the inspector.

### Fixed

- Removed nested ANSI styles from peer-table cell values. Those resets could
  interfere with the table's selected-row background and incremental terminal
  repaint, producing fragmented highlights and apparently displaced borders.
- Added an invariant across every wide width from 135 through 260 columns that
  all four vertical split-panel borders stay at identical positions on every
  rendered content row.

## [0.2.1] - 2026-08-08

### Fixed

- Peer results are tied to both the selected server and a request generation,
  preventing stale asynchronous responses from corrupting another server's
  screen or leaving it in a loading state.
- Opening Peers while discovery is still connecting now waits and starts the
  load automatically when the session is ready.
- A lost SSH ControlMaster connection triggers one bounded automatic reconnect;
  repeated failure settles into an actionable error instead of looping.
- SSH/sudo PTY failures retain their diagnostic output, so exit code 255 is no
  longer displayed with an empty explanation.
- Peer errors are sanitized and wrapped inside the panel.
- Table column calculations account for renderer cell padding. Wide seven-column
  tables no longer overwrite or break panel borders.

### Verification

- Added regressions for early tab switching, stale server/request messages,
  bounded reconnect, failed PTY diagnostics and long error wrapping.
- Added a pixel-width-equivalent table property check for every terminal width
  from 135 through 260 columns, including the reported 204-column layout.

## [0.2.0] - 2026-08-08

### Added

- A stable dashboard TUI with compact, standard and wide responsive modes.
- An explicit new-client setup screen for QR, private bundle export,
  configuration text, or safe deferred setup.
- Encrypted vault master-password rotation and automatic migration from older
  Linux/XDG and macOS vault locations.
- User-home WireGuard export bundles containing a secret `.conf` and QR PNG
  with restrictive directory/file permissions and no overwrite.
- Editable Google DNS defaults for new full-tunnel client profiles.
- Semantic status colors, persistent selected-peer context and scrolling
  diagnostics.

### Changed

- Server dashboards reuse one SSH session while switching tabs and make every
  password prompt identify the credential it needs.
- Peer tables prioritize columns by actual available width and preserve
  one-row cells for long names, endpoints and Unicode text.
- Short terminals use compact task-focused screens; terminals below `64×18`
  show an actionable size guard without losing application state.
- Resize transitions fully clear stale terminal rows and borders.

### Fixed

- Terminal restoration and mutation flows no longer crash on stray Bubble Tea
  messages or panics around interactive SSH/sudo prompts.
- SSH/sudo prompting, session cleanup, QR/profile export, dashboard refresh and
  selected-peer navigation regressions found during real-terminal testing.

### Verification

- Unit, integration-style model, responsive property, race and fuzz tests.
- `go vet`, Staticcheck, Semgrep and Go vulnerability analysis.
- GoReleaser archives/packages for Linux and macOS on amd64/arm64.

## [0.1.0] - 2026-08-07

- Initial private release with TUI, CLI and MCP front ends, remote discovery,
  WireGuard server setup, peer lifecycle, encrypted local storage and release
  packaging.

[0.4.0]: https://github.com/DMYTROSKORIN/wgtui/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/DMYTROSKORIN/wgtui/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/DMYTROSKORIN/wgtui/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/DMYTROSKORIN/wgtui/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/DMYTROSKORIN/wgtui/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/DMYTROSKORIN/wgtui/releases/tag/v0.1.0
