# Terminal interface

The TUI is the default `wgtui` interface. It is designed as a stable dashboard,
not a sequence of unrelated prompts: the server list remains visible beside the
working area on wide terminals, the active context is always named, and the
bottom bar always shows the relevant keys.

## Startup and credentials

1. The local vault master password is requested once, before alternate-screen
   mode starts. It decrypts local server metadata and client profiles only.
2. Opening a server establishes one dashboard SSH ControlMaster session.
3. SSH and remote sudo credentials are requested only when required and cached
   in memory for that dashboard session. They are never stored in the vault,
   argv, environment variables, logs, or terminal history.
4. Switching Overview / Peers / Diagnose with `Tab` reuses the connected
   session. It must never open a new terminal prompt.
5. Returning to the server list or exiting closes the session and removes its
   private ControlMaster socket directory.

`wgtui` refuses to start as root. All app-owned persistent state is kept in the
invoking user's `~/.wgtui-config/vault.json` (directory `0700`, encrypted file
`0600`). Older Linux/XDG and macOS vault locations are migrated automatically.

The master password can be changed after login with `P` on the server-manager
screen. The vault verifies the current password and atomically re-encrypts all
content with a fresh salt. A failed write leaves the old vault usable.

## Layout

- `64×18` is the supported minimum and `90×24` is recommended. Below the
  minimum, wgtui replaces the active UI with a size message and ignores input
  other than resize and global quit; the current screen and data are preserved.
- Compact mode uses one task at a time: a single-panel server list, a reduced
  overview, priority peer columns and concise action bars. Standard mode adds
  selected-peer context. Wide mode is enabled only when both width and height
  can hold the peer table and inspector.
- Server manager: server list on the left and selected connection details on
  the right in standard/wide mode; compact mode uses a single panel.
- Once a server is open, its name and connection state move into the header; the
  redundant Servers sidebar disappears. `Esc` returns to the server manager.
- Narrow terminals show a compact Peer / State / Last seen table. Medium terminals
  progressively add VPN IP, RX and TX and place a compact selected-peer card below.
- Wide terminals use two proportional panels: a peer table and a persistent Selected
  peer inspector. Endpoint is added to the table only when it still leaves a useful
  Peer / alias column. Cells are always truncated to one terminal row, never wrapped.
- The inspector shows VPN address, interface, endpoint, readable handshake age,
  traffic, full public key, hidden PSK presence, profile availability and ownership.
- Operations: action selection, a secret-free plan, explicit confirmation,
  progress, and a result. Revoke separately asks about server access, the local
  profile and metadata.
- Forms and action screens use the same full-window bordered layout and palette.
  List selection moves a fixed cursor; rows do not rotate or jump when navigating.
- Peer changes immediately reload the Peers view. A visible activity line distinguishes
  loading/refreshing from an idle screen.
- After creating a peer, wgtui asks how to configure the client instead of
  leaving the user at an informational screen. The choices are QR for a phone,
  a private profile+QR export bundle for another device, configuration text for
  advanced use, or finishing with the profile kept in the encrypted vault.
- Every managed peer whose encrypted profile is available has `Show QR code` and
  `Save profile + QR bundle` actions. A `DimaOffice_WireGuard` directory (`0700`)
  contains `DimaOffice.conf` and `DimaOffice_QR.png` (`0600`). Existing bundles are
  never overwritten; both files are secrets because both encode the private key.
- Color is semantic and never the only signal: cyan identifies navigation and
  focused structure, purple marks selection, green means connected/success,
  yellow means attention or unmanaged/read-only state, red means error/danger/
  secret exposure, and muted gray is secondary context. Symbols and labels
  remain present for monochrome and low-color terminals.
- Resize events that shrink the terminal or cross a layout breakpoint force a
  full redraw, preventing stale borders and duplicated action bars. Diagnostic
  results scroll inside the available panel instead of overflowing it.

## Client DNS default

New client profiles default to Google Public DNS: `8.8.8.8, 8.8.4.4`. The value is
visible and editable in the add-peer form and through `--dns`. An explicit DNS is
important for full-tunnel profiles because the client should not depend on a resolver
reachable only through its previous physical-network route. Rotation preserves an
existing profile's DNS and supplies the Google default when the old profile had none.

## Keyboard contract

| Key | Meaning |
| --- | --- |
| `?` | Show keyboard help from dashboard and peer screens |
| `n` | Add a server or peer in the current context |
| `e` | Edit the selected server; shortcut to export from the new-client setup screen |
| `r` | Explicitly refresh the active view |
| `Enter` | Open the selected server or peer actions |
| `Tab` | Switch dashboard tabs; never request credentials by itself |
| `Esc` | Cancel the current dialog or return one level |
| `Ctrl+C` | Close sessions, restore the terminal and quit from every screen |

`Ctrl+C` is handled before any embedded form sees it, so its meaning does not
change based on focus. Destructive operations cannot use it as confirmation.

Function keys remain compatibility aliases for existing users, but no workflow or
footer requires them.

## Preshared keys

`PresharedKey` is optional WireGuard key material, not a replacement for `PublicKey`.
When enabled, wgtui generates a fresh 256-bit symmetric key locally and places the same
secret in that peer's server block and encrypted client profile. WireGuard mixes it into
the normal public-key handshake as an additional layer; WireGuard remains secure without
it. Treat it like a private key: never publish, log, or paste it into an untrusted service.
The add-peer form recommends it but lets the user turn it off.

Protocol reference: <https://www.wireguard.com/protocol/>.

## Test contract

Automated tests cover global quit from every screen, session cleanup, Tab using
the existing connection, borrowed-session survival across confirmation,
responsive rendering at every width from 64 through 180 across short/normal/tall
terminals, shrink and breakpoint redraws, tiny-terminal fallbacks, diagnostic
scrolling, post-create client setup, QR height safety, keyboard help, peer
readability, terminal-control sanitization fuzzing, export-path fuzzing, atomic
vault password rotation, stable action cursors, portable vault migration, and
preservation of encrypted data. Network mutations remain
covered by the transaction and peer-operation tests; final compatibility checks
must use a disposable WireGuard test server, never a production target.
