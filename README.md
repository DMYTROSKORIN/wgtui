# wgtui

Terminal UI and CLI for managing one or more remote WireGuard servers over plain SSH —
no agent, no web UI, no extra open port on the server.

> **Status: stable personal release.** The TUI, CLI, MCP front end, encrypted local
> vault, discovery, setup and peer lifecycle are implemented. See
> [`docs/SPEC.md`](docs/SPEC.md) for requirements, [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
> for internals and [`docs/TUI.md`](docs/TUI.md) for the terminal workflow.

---

## What it does

- Connects to a server over SSH (key, `ssh-agent`, or interactive password).
- Discovers WireGuard and existing interfaces, read-only, before touching anything.
- Shows interface/peer state, handshake, endpoint, and traffic counters.
- Generates client keypairs **locally** and hands the server only the public key — a
  ready `.conf` and QR code are produced on your machine, no `scp` involved.
- Gives new client profiles an editable Google DNS default (`8.8.8.8`, `8.8.4.4`),
  so full-tunnel clients do not silently depend on their previous network resolver.
- Adds, pauses, resumes, rotates, and revokes peers, each as a locked,
  verified, rollback-on-failure transaction with an ephemeral recovery copy.
- Offers a guided first-time server setup when no WireGuard interface exists yet.
- Leaves nothing but standard WireGuard, standard config files, and SSH on the server.

## Why

Managing a plain WireGuard server usually means hand-editing
`/etc/wireguard/wg0.conf` over SSH, or installing a whole extra panel just to add a peer.
`wgtui` keeps the server exactly as WireGuard ships it and puts the tooling on the machine
you already trust to administer it from.

## Non-goals for v1

No web UI, no cloud control plane, no reimplementation of the WireGuard protocol, no
mesh networking, no mandatory server daemon. Full list: [`docs/SPEC.md` §3.2](docs/SPEC.md).

<details>
<summary>Supported platforms (MVP)</summary>

**Control client:** Linux amd64/arm64, macOS amd64/arm64.

**Managed server:** Ubuntu, Debian, Fedora (systemd). macOS is a client-only platform in
this project — it is never a managed server.

</details>

<details>
<summary>Security posture</summary>

- Server private key never leaves the server; client private keys never touch the server.
- Strict SSH host-key verification by default, always.
- Every mutating operation uses a locked temporary recovery copy, verifies the
  result, and rolls back on failure. The copy is retained only if recovery itself fails.
- No secret is ever passed as a process argument, logged, or included in `--json` output.
- App-owned local state lives in `~/.wgtui-config/vault.json`, encrypted under the
  master password. An explicit TUI export creates a `0700` bundle in the invoking
  user's home with a profile and QR PNG, both mode `0600`.
- `wgtui` refuses to run as root so local keys cannot silently move into root's home.

Full analysis: [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md).

</details>

---

## Building

```bash
go build ./...
```

Running `wgtui` without arguments opens the full-screen dashboard. The master
password is requested once before the dashboard starts. Inside the app, `F9`
opens the complete command menu, `F1` shows keyboard help, `F5` refreshes,
`Esc` goes back, and `F10`/`Ctrl+C` exits safely. The minimum interactive
terminal size is `64×18`; `90×24` or
larger is recommended. The interface switches between compact, standard and
wide layouts automatically and preserves the current screen while the terminal
is temporarily too small.

<details>
<summary>Installing a release</summary>

This is a private repo for personal use, not distributed to third parties - releases are
`.deb`/`.rpm`/`.tar.gz` on GitHub Releases only (`.github/workflows/release.yml`, triggered
by a `vX.Y.Z` git tag), no Homebrew tap and no GPG signature (see `.goreleaser.yaml` for the
reasoning).

```bash
scripts/install.sh          # latest release, or: scripts/install.sh vX.Y.Z
```

The recommended way to install: no root, ever. Downloads via `gh` (needed either way - the
repo is private, so a plain `curl` of the asset URL can't authenticate), verifies
`checksums.txt`, and installs the binary, man page, and shell completions entirely under
`~/.local/` - the same user-owned-prefix approach as rustup/pipx/cargo, not a system package
manager. Re-run it after every release to update.

`.deb`/`.rpm` remain available if you specifically want a system-managed install instead
(these do need root, because that's what a system package manager is):

```bash
sudo dpkg -i wgtui_*.deb   # Debian/Ubuntu
sudo rpm -i wgtui_*.rpm    # Fedora
```

`wgtui version` checks GitHub Releases for a newer version and prints a one-line notice if
one exists; it never downloads or installs anything itself - `scripts/install.sh` is always
a deliberate, separate step.

</details>

## Development

Every push is scanned with [Semgrep](https://semgrep.dev) (`p/golang`, `p/security-audit`,
`p/secrets`) — enforced server-side in CI (`.github/workflows/ci.yml`) and, once you've run
the one-time setup below, locally too:

```bash
scripts/install-git-hooks.sh   # once per clone: enables the pre-push semgrep gate
scripts/semgrep-scan.sh        # run it manually any time
```

## License

MIT — see [LICENSE](LICENSE).
