# wgtui

Terminal UI and CLI for managing one or more remote WireGuard servers over plain SSH —
no agent, no web UI, no extra open port on the server.

> **Status: early development.** Only the project skeleton exists so far. See
> [`docs/SPEC.md`](docs/SPEC.md) for the full specification and
> [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for how it's being built.

---

## What it does

- Connects to a server over SSH (key, `ssh-agent`, or interactive password).
- Discovers WireGuard and existing interfaces, read-only, before touching anything.
- Shows interface/peer state, handshake, endpoint, and traffic counters.
- Generates client keypairs **locally** and hands the server only the public key — a
  ready `.conf` and QR code are produced on your machine, no `scp` involved.
- Adds, pauses, resumes, rotates, and revokes peers, each as a backed-up,
  verified, rollback-on-failure transaction.
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
- Every mutating operation is a locked backup → apply → verify → rollback-on-failure
  transaction.
- No secret is ever passed as a process argument, logged, or included in `--json` output.

Full analysis: [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md).

</details>

---

## Building

```bash
go build ./...
```

<details>
<summary>Installing a release (once one exists)</summary>

Every push to `master` is scanned and tested, but no version has been tagged yet - the
`.deb`/`.rpm`/Homebrew paths below describe what `wgtui vX.Y.Z` will ship once one is
(`.github/workflows/release.yml`, triggered by a `vX.Y.Z` git tag). Track progress in the
[Stage 5 issue](https://github.com/DMYTROSKORIN/wgtui/issues/6).

```bash
# Homebrew (macOS/Linux)
brew install DMYTROSKORIN/tap/wgtui

# Debian/Ubuntu
sudo dpkg -i wgtui_*.deb

# Fedora
sudo rpm -i wgtui_*.rpm
```

Every release is checksummed (`checksums.txt`) and GPG-signed (`checksums.txt.sig`) - see
the verification steps in each release's notes. `wgtui version` checks GitHub Releases for
a newer version and prints a one-line notice if one exists; it never downloads or installs
anything itself.

Shell completion and a man page ship in every package above, or can be generated directly
from a `wgtui` binary you've already built:

```bash
wgtui completion bash > /etc/bash_completion.d/wgtui   # or zsh / fish
man docs/man/wgtui.1                                     # from a source checkout
```

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
