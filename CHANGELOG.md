# Changelog

All notable release changes are documented here. Versions follow semantic
versioning while the project remains below 1.0.

## [0.5.9] - 2026-08-10

Reliability fixes found during live verification against a target with
`sudo`'s `pwfeedback` option enabled (masked per-character password echo) -
not exercised by any previously-tested server.

### Fixed

- A remote target with `Defaults pwfeedback` in its sudoers echoes one `*`
  per password character while it's "typed", then erases that masked line
  with backspace/space control bytes once submitted. This artifact landed
  at the start of any `RunSudo`/`RunSudoScript` capture and was not
  stripped, so it got silently embedded as garbage content at the top of
  the next config `peer add`/`configure`/other mutating operations wrote -
  failing remote `wg-quick` validation every time on such a target.
  `internal/sshtransport.normalizePTYCapture` now recognizes and strips
  this specific artifact shape (asterisks immediately followed by at least
  as many backspace bytes) while leaving ordinary output that happens to
  start with literal asterisks untouched.
- Every transaction failure (validate failed, apply failed and rolled
  back, fingerprint mismatch, or a genuinely unknown failure) displayed as
  a blank `transaction script did not report success (exit 1):` with no
  explanation, even though the transaction script always echoes one. Cause:
  `internal/transaction.Apply` read only the command's `Stdout`, but a PTY
  is a single combined stream - `sshtransport.interactiveResult` correctly
  routes a failed command's entire captured output into `Stderr` instead,
  which `Apply` never looked at. It now reads whichever field actually has
  content.

## [0.5.8] - 2026-08-09

Security-hardening release addressing the findings of a full internal
project audit. No critical or high-severity issues were found; these are
the low-severity and defense-in-depth fixes.

### Security

- The local vault now binds its Argon2 parameters and salt into the
  AES-256-GCM additional authenticated data (new on-disk format version 2).
  Previously those fields lived in the file's plaintext header outside the
  authentication tag; tampering with them now fails decryption. Version-1
  vaults are still read transparently and upgrade to version 2 on their next
  save. (Not a practical exploit before - a wrong parameter just produced a
  wrong key and a failed tag check - but proper defense in depth.)
- The WireGuard interface name is now validated (`^[a-zA-Z0-9_-]{1,15}$`)
  and the config path shell-quoted before being used in the config-read
  command (`peerops.ResolveInterface`/`FetchCurrentConfig`). The write path
  already validated; the read path did not, leaving a latent shell-injection
  gap for an operator's own `--interface` value or an interface name sourced
  from a compromised server. Consistent with the project's own SPEC §23.1.

### Fixed

- The `peer_add` MCP tool description promised it "returns a ready-to-import
  client profile", but by deliberate (safer) design the result contains no
  secret - the profile is saved to the local vault and retrieved via `wgtui
  peer export`. Description corrected to match.

### Added

- After opening the WireGuard UDP port on a `ufw` backend, wgtui now warns
  when `ufw` itself is disabled (`ufw disable`): the rule is saved but has no
  effect until `ufw enable` is run. `systemctl is-active ufw` reports the
  service as active even when the firewall is logically off, which otherwise
  left the port looking open when it was not.

## [0.5.7] - 2026-08-09

### Changed

- The "wgtui vX.Y.Z" brand/version line rendered as the very first line of
  nearly every screen (server list, help, every form, the vault-unlock
  screen, every credential prompt), directly under the top menu bar -
  exactly the clutter reported multiple times already. It now sits just
  above the bottom key/action bar instead, on every one of those screens.

## [0.5.6] - 2026-08-09

### Fixed

- The vault-unlock/create screen and every SSH/sudo/host-key credential
  prompt rendered two consecutive header lines that both said "wgtui
  vX.Y.Z" plus the same section title - the `v0.5.4` header decluttering
  fixed this for the server-detail screen specifically, but missed this
  second, actually more frequently seen instance (every server connection
  hits a credential prompt). Down to one header line on both screens.

## [0.5.5] - 2026-08-09

### Fixed

- The MCP server's `list_servers` and `peer_list` tools returned a bare Go
  slice from their handler, which reflects to an `outputSchema` of
  `"type": "array"` instead of `"type": "object"`. MCP requires structured
  tool output to be an object, so a client enforcing that (confirmed:
  Claude Code) rejected the schema while listing tools with `Invalid
  input: expected "object"` - breaking discovery for every tool this
  server registers, not just the two offending ones, since `tools/list`
  returns them all in one response. Both now return a named wrapper struct
  (`ServerListResult`/`PeerListResult`) instead. A new test asserts every
  registered tool's `outputSchema` is object-shaped going forward.

## [0.5.4] - 2026-08-09

### Changed

- Version and connection status moved off the server-detail header row,
  which had three competing facts (brand+version, server name, connection
  state) crowding one line. The header is now just the server name; version
  and connection status live on the tabs row instead, right-aligned, staying
  visible on every tab instead of only Overview.
- Every list in the app now scrolls the same way: the selection cursor
  moves within a stable view, and the view only shifts once the cursor
  would go off the top or bottom. The peer list previously scrolled the
  whole view on every keypress instead - `bubbles/table`'s row window and
  its separate internal viewport scroll offset were both moving out of
  step, and the second one isn't reachable from outside the package to fix
  directly. wgtui now windows the peer list itself, the same way the
  server list and diagnose checklist already did (the server list also had
  no windowing at all before this - selecting a server past the visible
  area used to just render nothing there). As a side effect, clicking a
  peer row now works correctly even while the list is scrolled, which
  previously had to be a deliberate no-op for safety.
- After adding a peer, "Export profile + QR" is now the first, default
  option instead of "Show QR code" - it's the one used more often.

## [0.5.3] - 2026-08-09

### Fixed

- The vault password action's raw keyboard handler still had its own
  independent `P` binding untouched by the `v0.5.1` rename to `W`, so
  plain `P` kept opening the password form even though the UI said `W`.
  Now `W` only.
- Fixed a real rendering bug: the top-menu dropdown's horizontal offset was
  applied by prepending spaces to the whole multi-line panel string, which
  only indents the first line (the top border). Every other line - every
  item row, the bottom border - stayed at column 0, both visibly "sliding"
  apart from the border for every category except the first, and breaking
  the mouse click hit-test for those same rows.
- The server list and the peer table now respond to mouse clicks on their
  rows, the same as Up/Down already do. (The peer table only where every
  peer fits in the visible table height at once - `bubbles/table` exposes
  no way to read its internal scroll offset from outside the package, so a
  click while scrolled is intentionally a no-op rather than risking
  selecting the wrong peer.)
- A wrong sudo password was silently losing its own explanation: sudo's
  "Sorry, try again." retry text landed *before* the point later output
  gets trimmed from, so on final failure - especially when the remote side
  closed the connection right after giving up rather than printing a
  distinct summary line - nothing useful was left to detect, surfacing as
  an unexplained "exited 255" instead of a clear rejected-password message.
  Failed interactive SSH/sudo commands now return their entire captured
  output instead of the trimmed tail, and `RunSudo`/`RunSudoScript` (used
  by install, configure and peer operations, not just the initial connect)
  now detect and report a rejected sudo password the same way the initial
  connect already did in `v0.5.1`.
- Documented connecting Claude Desktop/Claude Code to `wgtui mcp` concretely
  (an actual config JSON example) and added an explicit, honest note on the
  security trade-off of putting the vault master password in a persistent
  client config file versus wgtui's normal memory-only handling.

## [0.5.2] - 2026-08-09

### Fixed

- Found while verifying `v0.5.1`'s own fixes: with a top-menu dropdown already
  open (e.g. from an earlier `M` press or `Alt+S`), pressing a *different*
  category's `Alt+letter` did nothing at all - not a terminal quirk, a real
  gap where that keypress fell through every case unmatched. This is almost
  certainly what "the top-menu hotkeys don't respond" actually was. `Alt+letter`
  now switches categories directly even while a dropdown is already open.

## [0.5.1] - 2026-08-09

### Fixed

- Overview, Peers and Diagnose tabs are now mouse-clickable, matching every
  other menu/action segment in the app.
- Fixed a real display bug in the wide Diagnose panel: the selected check
  row's status badge is already ANSI-styled text, and truncating it with the
  helper meant for plain user text stripped the ESC bytes out of its color
  codes while leaving the rest of each escape sequence behind as visible
  garbage (e.g. `[1;38;2;255;95;109m`). The selected row now builds from
  plain text and is styled exactly once.
- The vault password action moved from `P` to `W` (pass**W**ord) on the
  server list, so it can never be confused with `Alt+P` (the Peer menu) -
  reported as confusing in practice, especially on terminals where an
  Alt-prefixed sequence occasionally arrives as a split Escape-then-letter.
  No bottom-bar accelerator, on any screen, is ever a top-menu category's
  own letter now (enforced by a new test).
- SSH and sudo passwords are now cached for as long as a server stays open in
  the detail screen, not just for a single connection - install, post-install
  rediscovery and configure no longer ask for the same password repeatedly
  within one working session. The cache is cleared when the server session
  ends and specifically when a cached sudo password turns out to be wrong,
  so a rejected password is never silently retried.
- A wrong sudo password now produces an explicit "sudo password was rejected
  by the server" message instead of silently falling back to a generic
  "insufficient privileges" state with no indication of what actually went
  wrong.
- Installing WireGuard on a server with no existing interface now continues
  straight into the configure wizard instead of stopping at Overview with
  Configure as a separate, easy-to-miss step.
- Documented the already-implemented `wgtui mcp` AI-agent/automation front
  end in the README - it existed and worked, but had no discoverable usage
  instructions outside `docs/ARCHITECTURE.md`.

## [0.5.0] - 2026-08-09

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
