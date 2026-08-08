# Commander UI contract

`wgtui` keeps its Go business logic and Bubble Tea renderer while adopting the
interaction model that makes orthodox file managers such as Midnight Commander
discoverable and stable. This is a behavior contract, not a visual clone and
not a dependency on S-Lang or curses.

## Goals

- Every normal action is discoverable through a menu; memorized shortcuts are
  accelerators, never the only route.
- Main screens retain a stable two-pane geometry: collection/task context on
  the left and selected-object details on the right.
- The top menu and bottom letter-action bar remain in fixed rows. Async state
  changes update content, not the overall frame.
- Linux and macOS use the same pure-Go code and keyboard model. Release builds
  remain `CGO_ENABLED=0` for Linux/macOS on amd64/arm64.
- Color reinforces state but text, symbols and selection remain sufficient in
  monochrome terminals.

## Top menu

The persistent menu categories are:

| Menu | Responsibilities |
| --- | --- |
| Server | Open, add, edit, remove, install WireGuard, configure, refresh |
| Peer | Add, inspect/actions, QR, export, pause/resume, rotate, revoke, refresh |
| View | Overview, Peers, Diagnose, refresh current view |
| Tools | Change vault password and future maintenance actions |
| Help | Keyboard help, version/about, quit |

`M` opens the command menu. Left/right switches category, up/down selects an
item, Enter executes it, and Esc closes it without changing the underlying
screen. Every category and item is mouse-clickable. Unavailable actions remain
visible with a reason instead of silently disappearing.

## Letter-action bar

The bottom row is divided into high-contrast segments. Each action exposes a
highlighted letter, matching btop's discoverable shortcut convention:

| Key | Stable meaning |
| --- | --- |
| ? | Help |
| N | New server or peer |
| O | Open selected server |
| E | Edit selected server |
| R | Refresh current data |
| C | Configure server when applicable |
| D | Diagnose/view diagnostics |
| A | Selected peer actions |
| X | Remove server or revoke peer |
| M | Menu |
| Q | Quit safely |

At the supported minimum `64×18`, labels may abbreviate but accelerator letters
remain visible. Wider terminals use descriptive labels. Each segment is clickable.

## Screen model

### Servers

- Left: registered servers and selection.
- Right: target, authentication method and a concise next-action explanation.
- Enter/O opens the selected server; N/E/X create/edit/remove.

### Server detail

- Overview: readiness and configuration facts on the left, recommended actions
  and important warnings on the right.
- Peers: peer table on the left, selected peer inspector on the right.
- Diagnose: check list on the left, selected check detail/remediation on the
  right where space permits.

### Dialogs

Forms and destructive confirmations remain explicit modal tasks. They show the
operation and target, provide an Esc cancel path, and never overload a menu key
with immediate destructive behavior.

## Rendering invariants

- Every rendered line is at most the terminal width.
- Panel borders stay at identical columns for every content row.
- Table values contain no embedded ANSI styling; the table owns row selection.
- Resizing across a layout breakpoint clears the renderer before repainting.
- Async results are scoped to their server and request generation.

## Verification

- Exhaustive width/height layout tests, including the supported minimum.
- Menu reachability and disabled-action tests for every screen context.
- Letter-accelerator and mouse dispatch tests plus compact-label bounds.
- ANSI-free table rows and fixed-border position properties.
- Race tests for async menu-triggered refreshes.
- Linux and Darwin cross-builds for amd64 and arm64 in every release.
