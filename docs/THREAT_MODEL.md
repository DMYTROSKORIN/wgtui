# Threat model

Scope: `wgtui` control client and the standard WireGuard/SSH surface it manages on a
remote server. See [`docs/SPEC.md`](SPEC.md) §23 for the baseline security requirements
and [`docs/ARCHITECTURE.md`](ARCHITECTURE.md) for the component map referenced below.

## Assets

| Asset | Where it lives | Why it matters |
|---|---|---|
| Server WireGuard private key | Remote server only | Compromise lets an attacker impersonate the server and decrypt/inject traffic for every peer |
| Client WireGuard private keys | Local profile vault only | Compromise lets an attacker impersonate that one client |
| SSH credentials (key, agent, or interactive password) | Local machine / ssh-agent / Keychain | Compromise gives shell/root access to the managed server |
| `sudo` password | Held in process memory only during a session | Compromise gives root on the managed server |
| Master password for the profile vault | User's memory + derived key in process memory | Compromise decrypts every stored client profile |
| WireGuard config and temporary recovery copy | Remote server, `/etc/wireguard` | Contains the server private key; must never be copied off-host unredacted |
| Local profile vault contents | Local machine, encrypted at rest | Contains every client private key ever issued from this machine |

## Trust boundaries

1. **Local machine ↔ remote server**, crossed only over the SSH transport
   (`internal/sshtransport`).
2. **wgtui process ↔ disk**, crossed by `internal/profilevault` (encrypted) and
   `internal/wgconfig`/`internal/transaction` (remote config files and temporary
   recovery copies).
3. **wgtui process ↔ terminal**, crossed by CLI/TUI output, logs, and the QR display —
   anything rendered here is implicitly "shown to whoever is looking at this screen or
   this terminal's scrollback."

## Threats and mitigations

### T1 — Server private key exfiltration via config read

*Attack:* A bug in discovery/inspection prints or transmits the full
`/etc/wireguard/<iface>.conf`, leaking `PrivateKey` to the local machine, a log file, or
JSON output.

*Mitigation:* `internal/wgconfig` redactor is the single mandatory chokepoint (spec §14,
§29.11) for any config text leaving the remote host. `wg show ... dump` is the preferred
read path since it never includes the interface private key. Security tests (spec §26.4)
assert the key never appears in stdout/stderr/debug logs across the whole read path, not
just at the call site that first reads the file.

### T2 — Secret leakage via process arguments or environment

*Attack:* A private key, SSH password, or `sudo` password appears in `ps aux` output, the
shell history, or a child process's environment inherited by something else.

*Mitigation:* Secrets are only ever passed via stdin or an interactive PTY (spec §10.3,
§23.1). `internal/sshtransport` is the only package allowed to spawn `ssh`/`sudo`, and it
is built to make argument-based secret passing structurally impossible (no `Secret string`
field reaches `exec.Cmd.Args`).

### T3 — Command injection via user-controlled strings

*Attack:* A peer name, interface name, note, or path is used to build a remote shell
command and breaks out of the intended command.

*Mitigation:* No shell command is ever built by string concatenation of user input (spec
§23.1, §29.9). All remote invocations use argument arrays or safe stdin payloads with
strict quoting; every user-controlled value that reaches a remote adapter is validated
first (interface name, IP/CIDR, port, public key, path) per spec §23.1. Regression tests
cover shell metacharacters, terminal escape sequences, and oversized input in names/notes
(spec §26.4).

### T4 — TOCTOU during config mutation

*Attack:* Two concurrent `wgtui` invocations (or a manual edit) change the config between
when it's read and when it's written, silently discarding one change or corrupting state.

*Mitigation:* `internal/transaction` re-reads a fingerprint of the current config
immediately before writing and refuses to proceed if it has changed since the plan was
built (spec §15.2 step 3); an interface-scoped lock (`flock`, falling back to atomic
lock-directory creation, spec §15.3) serializes concurrent `wgtui` runs against the same
interface.

### T5 — Host key spoofing / MITM

*Attack:* An attacker on-path presents a different SSH host key, or a stale/poisoned
`known_hosts` entry lets a MITM through silently.

*Mitigation:* Strict host-key checking is always on; `StrictHostKeyChecking=no` is never
set (spec §10.2, §23.1). First-connect fingerprint is shown and requires explicit
confirmation. A host-key change is a hard stop with an explicit warning, never a silent
prompt-and-continue.

### T6 — Partial mutation leaves the server in a broken or inconsistent state

*Attack:* SSH drops mid-mutation, or an intermediate step fails, leaving persistent config,
runtime state, and local metadata disagreeing with each other — worst case, WireGuard
stops passing traffic or SSH itself becomes unreachable.

*Mitigation:* Every mutation creates a same-host temporary recovery copy before writing,
is verified after writing, and is rolled back on an apply failure (spec §15.2, §24).
The copy is removed after success or verified rollback and retained only when automatic
recovery itself fails. Reconnection after an SSH drop triggers a fresh
discovery/drift comparison rather than blindly retrying (spec §24.1). Firewall/NAT changes
never flush existing tables/chains (spec §13.5) — a full flush during a config mistake is
exactly the class of failure that can cut off the SSH session used to fix it.

### T7 — Local profile vault compromise

*Attack:* An attacker with read access to the local machine (stolen laptop, malware) reads
every client private key ever issued.

*Mitigation:* Vault is encrypted at rest with a master password set on first run (spec
§30.4), directory `0700`/files `0600` as defense in depth, atomic writes so a crash never
leaves a half-written plaintext profile. QR code rendering is called out explicitly as
displaying a secret and is never written to terminal scrollback history or a log (spec
§7.4).

### T8 — Malicious or malformed remote config

*Attack:* A remote server (compromised, or simply hand-edited by someone else) returns an
oversized, malformed, or crafted config file designed to exploit the parser or terminal
(escape sequences in a comment or peer name, for example).

*Mitigation:* Parser has size limits and is fuzz/golden tested against malformed and
oversized fixtures (spec §26.2, §26.4). Any remote-sourced string that reaches a terminal
UI is sanitized against escape-sequence injection before rendering.

### T9 — Unattended/incomplete peer revocation

*Attack:* A "delete" is misunderstood as fully destructive or partially applied, leaving
either server access alive when the user believed it revoked, or a private key silently
destroyed when the user believed it kept.

*Mitigation:* Revoke asks three separate, explicit questions — revoke server access,
delete local profile, delete metadata — with server revocation not implying local file
deletion (spec §17.5). No default silently does the more destructive thing.

### T10 — Credential and secret exposure via the MCP front end

*Attack surface:* `internal/mcpserver` (see docs/ARCHITECTURE.md) lets an AI agent drive
`wgtui` over the MCP stdio transport instead of a human at a terminal. Two properties of
that transport change the threat picture from T2/T5 above:

1. **Credentials as tool arguments, not terminal input.** Stdin/stdout *are* the MCP
   protocol channel, so there is no terminal left to prompt on (unlike cli/tui's
   `term.ReadPassword`). SSH and sudo passwords are instead passed per tool call as JSON
   arguments (`internal/mcpserver/credentials.go`'s `Credentials` struct) and used
   transiently in-memory — never cached across calls, never written to disk. T2's mitigation
   (never in `ps`/shell history/env) still holds for the process itself, but the value now
   necessarily passes through the calling agent's conversation/context to reach the tool
   call at all. That is a different exposure surface, not a strictly smaller or larger one:
   whether it's better or worse than a local terminal prompt depends on how the calling
   agent's own context is stored, logged, or retained — something `wgtui` has no visibility
   or control over. Tool descriptions explicitly instruct the calling agent to only pass a
   credential the user has just supplied for that specific call, never to invent one or
   reuse one from earlier in the conversation.
2. **No human to verify a host key out of band.** T5's mitigation (interactive fingerprint
   confirmation) has no terminal to render on and no human present to answer even if there
   were. `Credentials.prompts()` therefore hard-codes host-key confirmation to always refuse
   — a first-time connection to any server must be established once from a real terminal
   (`wgtui server inspect`, cli or tui) before that server can be driven through MCP. This
   makes MCP strictly unable to establish first trust in a host key; it can only reuse trust
   a human already established.

*Mitigation:* Every mutating tool is gated behind an explicit `confirm bool` argument
(default `false`, preview-only — computes the same plan a human would see, but changes
nothing and generates no keys); each tool's `Description` tells the calling agent never to
set `confirm=true` without the user having seen and approved a prior `confirm=false`
preview, mirroring cli's interactive `Apply? [y/N]` and tui's plan screen. Tool *results* are
kept secret-free by design: `peer_add`'s response never includes the generated private key
or rendered client profile (unlike cli's optional `--show`/`--qr`/`--output`) — retrieving
the actual profile file is left to the existing `wgtui peer export` CLI command, so no
`wgtui`-generated secret is ever placed in an MCP tool result or, by extension, an agent's
context, by default.

## Explicitly out of scope (spec §3.2, §12.2, §6.1)

- Compromise of the remote server's OS/kernel outside of what standard WireGuard + SSH
  hardening already covers — `wgtui` does not add its own daemon or listening port that
  could become a new attack surface, and does not attempt to harden the base OS.
- Protection against a fully malicious/root-compromised local machine — if the machine
  running `wgtui` is owned by an attacker, the master password and any unlocked vault
  contents are lost; this is the same trust assumption as any local secrets manager.
- Denial of service against the WireGuard UDP port itself.

## Status

Living document — update whenever a new mutating operation or remote adapter is added, and
whenever an assumption above is invalidated by an implementation decision.
