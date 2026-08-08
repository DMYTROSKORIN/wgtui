# Changelog

All notable release changes are documented here. Versions follow semantic
versioning while the project remains below 1.0.

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

[0.2.1]: https://github.com/DMYTROSKORIN/wgtui/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/DMYTROSKORIN/wgtui/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/DMYTROSKORIN/wgtui/releases/tag/v0.1.0
