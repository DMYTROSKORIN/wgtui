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

## Two accelerator systems

The top menu and the bottom bar look related but answer different questions,
and they must never make the same letter promise two different things:

- **Top menu (`Alt+letter`)** - "where do I navigate?" Calm and unobtrusive
  until opened; the accelerator is `Alt+` the category's first letter, stored
  explicitly per category rather than derived from its label.
- **Bottom bar (plain `letter`)** - "what can I do on this screen right now?"
  High-contrast and always visible; the accelerator is a plain letter that is
  always a real letter *inside* the action's own label (e.g. the `R` in
  `Remove`, the `O` in `revOke`) - never a separate key cell printed next to
  the label.

Because the two systems use different modifiers, a letter can safely be
reused between them - `Alt+P` always opens the Peer menu even while a bottom
bar uses plain `P` for something else. In practice no bottom-bar action uses
a top category's own letter (`S`/`P`/`V`/`T`/`H`) at all: the vault password
action is bound to `W` (passWord), not `P`, specifically so a bottom-bar
keypress can never be mistaken for the Alt+letter it superficially resembles,
even on a terminal where an Alt-prefixed sequence occasionally arrives split
into a lone Escape followed by a plain letter.

## Top menu

The persistent menu categories are:

| Menu | Accelerator | Responsibilities |
| --- | --- | --- |
| Server | `Alt+S` | Open, add, edit, remove, install WireGuard, configure, refresh |
| Peer | `Alt+P` | Add, inspect/actions, QR, export, pause/resume, rotate, revoke, refresh |
| View | `Alt+V` | Overview, Peers, Diagnose, refresh current view |
| Tools | `Alt+T` | Change vault password and future maintenance actions |
| Help | `Alt+H` | Keyboard help, version/about, quit |

`M` also opens the command menu (on the last-focused category) as a
mouse-free, modifier-free fallback. Left/right switches category, up/down
selects an item, Enter executes it, and Esc closes it without changing the
underlying screen. Every category and item is mouse-clickable. Unavailable
actions remain visible with a reason instead of silently disappearing.

No category shows an active background while the menu is closed - only the
open dropdown's own category does. Which screen or tab happens to be showing
is a separate fact from which menu is open, and the top row never conflates
the two. There is no `│` separator between categories at any supported width;
adjacent categories are told apart by their own text and even padding alone,
the same calm reading Midnight Commander's top row gives its own categories.

## Letter-action bar

The bottom row is divided into high-contrast segments sharing one continuous
background per segment - never a separate darker key cell next to the label.
Each action exposes a highlighted letter *inside* its label text, matching
btop's discoverable shortcut convention while reading as a single word:

| Key | Screen | Stable meaning |
| --- | --- | --- |
| ? | any | Help |
| N | any | New server or peer |
| O | server list | Open selected server |
| E | server list | Edit selected server |
| R | server list | Remove selected server |
| W | server list | Change vault passWord |
| R | server detail | Refresh current data |
| I | server detail | Install WireGuard when missing |
| C | server detail | Configure server when applicable |
| D | server detail | Diagnose/view diagnostics |
| A | Peers tab | Selected peer actions |
| O | Peers tab | Revoke selected peer |
| B | server detail | Back to server list |
| M | any | Menu |
| Q | any | Quit safely |

`O` and `R` mean different things on different screens - each bottom bar's
own accelerators are unique among themselves (case-insensitively), but the
same letter is free to be reused once its earlier screen is no longer
visible. A thin dark gap, not a `│`, separates adjacent segments.

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
