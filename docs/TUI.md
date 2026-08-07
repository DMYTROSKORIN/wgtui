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

- Server manager: server list on the left, selected connection details on the
  right.
- Server dashboard: medium terminals show server/session summary on the left and
  Overview, Peers or Diagnose on the right. Narrow terminals use one content panel.
- Wide Peers dashboard: three proportional panels show Servers, a full-height peer
  table, and a persistent Selected peer inspector. The table gains an Endpoint
  column when space permits; its other columns grow proportionally instead of
  leaving an unused rectangle on the right.
- The inspector shows VPN address, interface, endpoint, readable handshake age,
  traffic, full public key, hidden PSK presence, profile availability and ownership.
- Operations: action selection, a secret-free plan, explicit confirmation,
  progress, and a result. Revoke separately asks about server access, the local
  profile and metadata.
- Forms and action screens use the same full-window bordered layout and palette.
  List selection moves a fixed cursor; rows do not rotate or jump when navigating.
- Peer changes immediately reload the Peers view. A visible activity line distinguishes
  loading/refreshing from an idle screen.
- Every managed peer whose encrypted profile is available has `Show QR code` and
  `Save client profile to home directory` actions. Exports are mode `0600`, never
  overwrite an existing file, and accept a file name rather than an arbitrary path.

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
| `e` | Edit the selected server; save an available peer profile from its reveal screen |
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
responsive dashboard rendering, keyboard help, peer readability, atomic vault
password rotation, stable action cursors, portable vault migration, and preservation
of encrypted data. Network mutations remain
covered by the transaction and peer-operation tests; final compatibility checks
must use a disposable WireGuard test server, never a production target.
