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

The master password can be changed after login with `P` on the server-manager
screen. The vault verifies the current password and atomically re-encrypts all
content with a fresh salt. A failed write leaves the old vault usable.

## Layout

- Server manager: server list on the left, selected connection details on the
  right.
- Server dashboard: server/session summary on the left and Overview, Peers or
  Diagnose on the right. Narrow terminals use a single content panel.
- Peers: readable state, VPN address, handshake age, human-readable traffic,
  followed by a card explaining the selected peer and available profile.
- Operations: action selection, a secret-free plan, explicit confirmation,
  progress, and a result. Revoke separately asks about server access, the local
  profile and metadata.

## Keyboard contract

| Key | Meaning |
| --- | --- |
| `F1` | Show keyboard help from any screen |
| `F2` / `a` | Add a server or peer in the current context |
| `F4` / `e` | Edit the selected server |
| `F5` / `r` | Explicitly refresh the active view |
| `F8` / `Enter` | Open actions for the selected peer |
| `Tab` | Switch dashboard tabs; never request credentials by itself |
| `Esc` | Cancel the current dialog or return one level |
| `F10` / `Ctrl+C` | Close sessions, restore the terminal and quit |

`Ctrl+C` is handled before any embedded form sees it, so its meaning does not
change based on focus. Destructive operations cannot use it as confirmation.

## Test contract

Automated tests cover global quit from every screen, session cleanup, Tab using
the existing connection, borrowed-session survival across confirmation,
responsive dashboard rendering, keyboard help, peer readability, atomic vault
password rotation and preservation of encrypted data. Network mutations remain
covered by the transaction and peer-operation tests; final compatibility checks
must use a disposable WireGuard test server, never a production target.
