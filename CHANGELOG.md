# Changelog

All notable release changes are documented here. Versions follow semantic
versioning while the project remains below 1.0.

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

[0.2.0]: https://github.com/DMYTROSKORIN/wgtui/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/DMYTROSKORIN/wgtui/releases/tag/v0.1.0
