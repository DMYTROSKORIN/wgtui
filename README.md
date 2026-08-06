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

## License

MIT — see [LICENSE](LICENSE).
