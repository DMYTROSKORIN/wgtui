# Project checkpoint

Last updated: 2026-08-09

Active branch: `master`

Release target: `v0.5.8`

Release status: **releasing now** - security-hardening release fixing the
low-severity and defense-in-depth findings from a full project audit
(`~/Documents/wgtui-audit-2026-08-09.md`; no critical/high issues found)

This file is the durable restart point for work on `wgtui`. After an interrupted
session, read it before changing code, then confirm the actual repository state
with `git status --short --branch` and `git log -5 --oneline`.

## Completed in the `v0.5.8` release cycle

- Ran a full security/stability/UX/MCP audit of the project (report saved to
  `~/Documents/wgtui-audit-2026-08-09.md`). No critical or high-severity
  vulnerabilities. semgrep/staticcheck/govulncheck clean; all 24 gosec hits
  were false positives; `-race` clean; the wgconfig parsers survived ~5.5M
  fuzz executions with no panic. The install wizard was verified end-to-end
  live on the test VM (WireGuard fully removed with `apt-get purge`, then
  reinstalled through the TUI: install→configure→peer→QR→export), and the
  MCP layer was verified with the real `claude` CLI (confirm-gate really
  does not mutate at `confirm=false`; tool responses carry no secrets, only
  the public key). This cycle then fixed every finding the audit raised:
- **F4 (defense in depth):** the vault now binds its Argon2 parameters and
  salt into the AES-256-GCM AAD (new format version 2, `internal/profilevault`).
  Version-1 vaults still open and upgrade to version 2 on the next save.
  Verified live: the real v0.5.7-created audit vault (version 1) opened with
  the new binary and upgraded to version 2 after a change-password re-encrypt,
  and reopened cleanly through the AAD read path. Tests:
  `TestVault_WritesVersion2WithBoundParams`,
  `TestVault_TamperedArgonParamsFailToOpen`,
  `TestVault_LegacyVersion1StillOpens`.
- **F2 (hardening):** interface name validated (`^[a-zA-Z0-9_-]{1,15}$`) and
  config path shell-quoted before the config-read command in
  `peerops.ResolveInterface`/`FetchCurrentConfig`. The write path
  (`transaction/script.go`) and discovery already validated; the read path
  did not - a latent injection gap for an operator's `--interface` value or a
  server-sourced interface name (THREAT_MODEL T8). Tests in
  `internal/peerops/config_test.go`.
- **F1:** corrected the `peer_add` MCP tool description, which promised a
  client profile the result deliberately (and safely) omits.
- **F3:** after `ufw allow <port>/udp`, warn when ufw is disabled (`ufw
  disable`) - `systemctl is-active ufw` is "active" even then, so the rule
  was added but not enforced. Surfaced in both the TUI and CLI configure
  flows (`osadapter.UfwIsInactive`).
- **F5 (accepted residual, documented):** the master password is held as a
  Go string and cannot be zeroed in pure Go; documented in
  `docs/THREAT_MODEL.md` T7 alongside the SSH/sudo `[]byte` zeroing that
  *is* done.
- `gofmt`, `go build`, `go vet`, `go test ./...`, `go test -race`,
  `staticcheck`, `govulncheck`, semgrep all clean on the changed tree.

## Completed in the `v0.5.7` release cycle

- After `v0.5.6` shipped ("de-duplicate the header"), the user sent a
  screenshot (`~/Pictures/Error/Screenshot_20260809_153820.png`) of the
  server-list ("Server manager") screen with a red arrow drawn from the
  `wgtui v0.5.6  Server manager` line at the very top all the way down to
  the bottom action bar - i.e. "move this line down there", not just
  "stop duplicating it". `v0.5.4`/`v0.5.6` had only ever deduplicated or
  nudged this line by one row (into `tabsLine`'s trailing slot on the
  server-detail screen specifically); the actual, literal ask - move the
  brand/version chrome to the bottom of the screen - had never been done
  anywhere.
  - Every screen built from the shared `appHeader(section)` helper (server
    list compact/wide, `formScreen` - used by seven form screens:
    configure, add peer, revoke confirm, export form, change password,
    server add/edit, remove-server confirm -, help compact/wide, confirm,
    applying, peer-reveal, peers-tab-compact) rendered that line as the
    very first line of the screen, directly under the top commander menu
    bar. Fixed by swapping the concatenation order in every one of those
    call sites: content panel first, then the brand line, then the
    existing key/action bar - a pure reordering of already-computed
    strings, so total line counts (and therefore `fullPanelHeight`/
    `dashboardPanelHeight`'s reserved-row math and `anchorFooter`'s bottom
    anchoring) are unaffected.
  - The vault-access and credential-prompt screens build their own colored
    `chromeStyle` bar rather than calling `appHeader`, but had the exact
    same "brand line is the first line" placement - given the same
    call-everywhere-you-can-find-the-pattern lesson `v0.5.6` was
    specifically about, moved those too rather than waiting for a third
    bug report on the same theme.
  - Left `internal/tui/view.go`'s `viewServerHeader`/`tabsLine` trailing
    version+connection-badge placement on the server-detail screen as is -
    that screen's top row no longer contains the word "wgtui" at all (the
    `v0.5.4` fix already removed it), so it isn't an instance of this bug;
    revisit only if the user flags it specifically.
  - Live-verified in tmux (scratch `HOME`, fresh vault): vault-unlock
    screen, empty server list, add-server form and help screen all show
    the brand/version line directly above the bottom bar, never at the
    top.
  - Added `TestBrandVersionLineSitsAboveFooterNotAtTop`
    (`internal/tui/dashboard_test.go`): renders the server list, help,
    vault-access and credential-prompt screens and asserts the brand line
    is never the first non-blank line and is one of the last two.
    Confirmed via `git stash` (fix files only, keeping the new test) that
    it fails against the pre-fix code with exactly the failure shape the
    screenshot showed, and passes with the fix.
- `gofmt`, `go build`, `go vet`, `go test ./...`, `go test -race ./...`,
  `staticcheck`, `govulncheck` all pass on the changed tree.
- Pushed `master` directly (commit `43b720d`); the pre-push Semgrep gate
  (101 rules, 116 files) was clean.
- Annotated tag `v0.5.7` points at `43b720d` and is pushed to origin.
  Release workflow run `31316708774` passed all three jobs (`semgrep`,
  `validate`, `goreleaser`).
- GitHub Release `v0.5.7` is published with `checksums.txt` and the usual
  Linux/macOS archives, `.deb` and `.rpm`.
- `scripts/install.sh v0.5.7` downloaded, checksum-verified and installed
  `wgtui 0.5.7` at `~/.local/bin/wgtui`.
- Installed `v0.5.7` binary SHA-256:
  `d4e7c2ad324d386cbed07f4193b3f782b0bb61f1e6981fd14fc3ed660ff17158`.
- Live-verified against the actual installed binary (not just the dev
  build used earlier), scratch `HOME`, fresh vault: server list screen
  shows `wgtui v0.5.7  Server manager` directly above the bottom action
  bar - the same screen and scenario the user's screenshot showed broken.

## Completed in the `v0.5.6` release cycle

- The user reported that "version/status moved to the bottom" (the header
  decluttering claimed complete in the `v0.5.4` cycle below) still wasn't
  done - a direct, fair challenge, since I'd reported it as finished.
  Investigation showed both things were true at once: the original
  `v0.5.4` fix genuinely *was* in place for the server-detail screen
  (`viewServerHeader`/`tabsLine` in `internal/tui/view.go`, still correct),
  but I had never checked whether the same "two lines both saying `wgtui
  vX.Y.Z · <title>`" clutter pattern existed anywhere else. It did: the
  vault-unlock/create screen (`viewVaultAccess`) and every SSH/sudo/host-key
  credential prompt (`viewCredentialPrompt`) each built a colored top bar
  with brand + version + section title, then rendered a second, separate
  `appHeader(...)` line repeating the same three facts directly underneath
  it. This is arguably the more frequently seen instance of the two, since
  every server connection hits a credential prompt.
  - Root cause: I fixed one occurrence of a pattern and reported the
    pattern itself as resolved, without grepping for other call sites of
    `appHeader` sharing the same "chrome bar + appHeader" shape. Recorded as
    a recurring failure mode to check for on future bug reports (see
    `CLAUDE.md`'s testing section and the "Resume here" note below).
  - Fix: removed the redundant `appHeader(title)` / `appHeader("Security")`
    line from `internal/tui/credentials.go`'s `viewCredentialPrompt` and
    `internal/tui/vault_access.go`'s `viewVaultAccess` - each screen now
    renders exactly one header line.
  - Live-verified in tmux (scratch `HOME`, fresh vault): both the
    vault-unlock screen and a triggered SSH-password prompt now show a
    single `wgtui vX.Y.Z · <title>` line, not two.
  - Added `TestAccessAndCredentialHeadersAreNotDuplicated`
    (`internal/tui/access_test.go`): renders both screens, counts lines
    starting with `wgtui`, asserts exactly one on each. Confirmed via `git
    stash` that it fails against the pre-fix code and passes with the fix.
- `gofmt`, `go build`, `go vet`, `staticcheck`, `go test ./...`,
  `go test -race ./...`, `govulncheck`, Darwin cross-builds and
  `goreleaser check` all pass on the changed tree.
- Pushed `master` directly (commit `7a4ee0a`); the pre-push Semgrep gate
  (101 rules, 116 files) was clean.
- Annotated tag `v0.5.6` points at `7a4ee0a` and is pushed to origin.
  Release workflow run `31314741212` passed all three jobs (`validate`,
  `semgrep`, `goreleaser`).
- GitHub Release `v0.5.6` is published with `checksums.txt`, four
  Linux/macOS archives, one `.deb` and one `.rpm`.
- `scripts/install.sh v0.5.6` downloaded, checksum-verified and installed
  `wgtui 0.5.6` at `~/.local/bin/wgtui`.
- Installed `v0.5.6` binary SHA-256:
  `afe371d66326dd1364946038105b70cddb4198c6c154a1d0416cafa00539a64f`.

## Completed in the `v0.5.5` release cycle

- Fixed a real MCP server bug the user hit trying to actually use `wgtui
  mcp` from Claude Code, right after the README's new MCP walkthrough was
  published: `list_servers` and `peer_list` handlers returned a bare Go
  slice (`[]ServerSummary`, `[]peerview.Peer`), which `mcp.AddTool`'s
  reflection-based schema inference turns into an `outputSchema` of
  `"type": "array"`. MCP requires structured output to be an object -
  Claude Code's client enforces that and rejected the schema while listing
  tools (`Invalid input: expected "object"`), which broke discovery for
  every tool this server registers at once, since `tools/list` returns
  them all in a single response - not just the two offending tools.
  - Root cause found by reading `github.com/modelcontextprotocol/go-sdk`'s
    `AddTool`/schema-inference source directly (`~/go/pkg/mod/.../mcp/server.go`)
    after noticing `listServers`/`peerList` were the *only* two handlers in
    `internal/mcpserver` returning a bare slice - every other tool already
    returned a named struct.
  - Fix: wrap both in a named result struct (`ServerListResult{ Servers
    []ServerSummary }`, `PeerListResult{ Peers []peerview.Peer }`) instead
    of returning the slice directly.
  - Added `TestToolOutputSchemasAreObjects`
    (`internal/mcpserver/server_test.go`): connects a real in-memory MCP
    client/server pair, calls `ListTools`, and asserts every tool's
    `outputSchema["type"]` is `"object"` (or absent) - exercises the actual
    wire protocol encode/decode path, not just the Go handler functions
    directly, so it would have caught this before it shipped.
- `gofmt`, `go build`, `go vet`, `staticcheck`, `go test ./...`,
  `go test -race ./...` pass on the changed tree.
- Pushed `master` directly (commit `c2fada6`); the pre-push Semgrep gate (101
  rules, 116 files) was clean.
- Annotated tag `v0.5.5` points at `c2fada6` and is pushed to origin. Release
  workflow run `31313747640` passed all three jobs (`semgrep`, `validate`,
  `goreleaser`).
- GitHub Release `v0.5.5` is published (not draft/prerelease) with
  `checksums.txt`, four Linux/macOS archives, two `.deb` files and two `.rpm`
  files.
- `scripts/install.sh v0.5.5` verified the published Linux amd64 archive and
  installed `wgtui 0.5.5` at `~/.local/bin/wgtui`.
- Installed `v0.5.5` binary SHA-256:
  `9fefc280e795f19c171edfd4de0c035b80663e86bb48d00ffb4c86b3dc4974a3`.
- **Verified the actual fix with the real `claude` CLI, not just the Go
  unit test**, after the user directly asked why this hadn't been done
  already (fair question - the earlier check only exercised the go-sdk's
  own in-memory client, a different code path than a real MCP client
  actually uses). Registered wgtui as a project-scoped MCP server pointing
  at a throwaway wrapper script + an isolated demo vault (never the user's
  real one), and ran `claude mcp list` - the exact command the user used
  to find the bug - against the pre-fix `v0.5.4` binary first, reproducing
  the identical error (`tools fetch failed — Invalid input: expected
  "object" ... (+1 more)`, matching exactly the two fixed tools). Re-ran
  the same command after installing `v0.5.5`: clean `✔ Connected`, no
  error. Removed the test registration afterward
  (`claude mcp remove wgtui-test`).

## Completed in the `v0.5.4` release cycle

- Moved version/connection status off the server-detail header row onto the
  tabs row (right-aligned, via a new `detailTrailing`/`tabsLine` trailing
  parameter) - three competing facts (brand+version, server name,
  connection state) on one line read as clutter. Verified live in tmux
  across compact, standard and wide layouts and all three tabs.
- Unified scroll behavior across every list in the app (server list, peer
  list, diagnose checklist): the selection cursor moves within a stable
  view, the view only shifts once the cursor would go off the top or
  bottom.
  - Root cause of the peer list specifically "moving as a whole": bubbles/
    table@v1.0.0's row-window (`start`/`end`) and its separate internal
    `viewport` scroll offset (`YOffset`) both move when `MoveUp`/`MoveDown`
    run, and the second one has no public accessor - unreachable to
    correct from outside the package. Tried driving the cursor via the
    public `SetCursor` alone first (bypasses `MoveUp`/`MoveDown` entirely);
    confirmed via `TestPeerTableDownStaysInSyncWithCursorWhenScrolled`
    that this alone is *not* sufficient (`viewport.YOffset` never moves,
    so the window stops following the cursor after a few presses).
    Settled on wgtui owning its own scroll window
    (`model.peerCursor`/`peerWindowStart`) and feeding `peerTable` only the
    current window's rows, sized to fit exactly - the same windowing
    `internal/tui/diagnose.go`'s list already used. `peerTable.Cursor()` is
    no longer used anywhere; `peerCursor` is the one source of truth.
  - `viewServerList` had no windowing at all before this - selecting a
    server past the visible area used to just silently render nothing
    there (`panelStyle`'s `Height()` clips, it doesn't scroll). Added the
    same windowing, shared between the renderer and the mouse click
    hit-test via `serverListRows`/`serverListVisibleRows`.
  - A real side effect: peer row clicks now work correctly even while the
    list is scrolled (`peerTableDataRowAt` used to be deliberately a no-op
    in that case, since bubbles/table's scroll position wasn't readable to
    verify against).
- `clientSetupChoices` reordered: "Export profile + QR" first (the option
  used more often), "Show QR code" second.
- New regression tests: `TestServerHeaderRowIsJustTheServerName`,
  `TestTabsRowCarriesVersionAndConnectionStatus`,
  `TestPeerTableDownStaysInSyncWithCursorWhenScrolled`,
  `TestServerListStaysInSyncWithCursorWhenScrolled`,
  `TestPeerRowClickWorksEvenWhileScrolled`,
  `TestClientSetupExportIsTheDefaultFirstChoice`.
- `gofmt`, `go build`, `go vet`, `staticcheck v0.7.0`, `govulncheck v1.6.0`
  (0 called vulnerabilities), `go test ./...`, `go test -race ./...`,
  `CGO_ENABLED=0` Darwin amd64/arm64 cross-builds, and `goreleaser check`
  all pass on the changed tree.
- Pushed `master` directly (commit `23f0b39`); the pre-push Semgrep gate (101
  rules, 114 files) was clean.
- Annotated tag `v0.5.4` points at `23f0b39` and is pushed to origin. Release
  workflow run `31310271718` passed all three jobs (`semgrep`, `validate`,
  `goreleaser`).
- GitHub Release `v0.5.4` is published (not draft/prerelease) with
  `checksums.txt`, four Linux/macOS archives, two `.deb` files and two `.rpm`
  files.
- `scripts/install.sh v0.5.4` verified the published Linux amd64 archive and
  installed `wgtui 0.5.4` at `~/.local/bin/wgtui`.
- Installed `v0.5.4` binary SHA-256:
  `f03b3be8b24a4d9bffbd00605e063c4514c7c5d73b85607747c89881f1ea4af0`.
- Documentation overhaul in the same session (not tied to a version release -
  no code changed, nothing to compile or tag):
  - Rewrote `README.md` for a genuinely public audience: what wgtui is, why
    it's different from wg-easy/Netmaker/Tailscale-style tools, who it's
    for, plus a demo GIF.
  - Removed the stale, now-fully-implemented `docs/UI_MENU_REDESIGN_HANDOFF.md`.
  - Softened "MVP" wording in `docs/ARCHITECTURE.md`/`docs/THREAT_MODEL.md`
    that read as "unfinished" to an outside reader (`docs/SPEC.md` itself
    was deliberately left alone - it's an internal, Russian-language formal
    spec hundreds of code comments cite by section number, not a
    public-facing document).
  - Added `CLAUDE.md` at the repo root: the onboarding doc for an AI agent
    picking up this project (architecture summary, non-negotiable rules,
    build/test/release commands, the live-tmux-verification testing
    convention this session leaned on heavily, and a pointer back to this
    file for current state).
  - Recorded `docs/assets/demo.gif` with `vhs` (`go install
    github.com/charmbracelet/vhs@latest`; needed `ttyd` too - no dnf/sudo
    available in-session, so downloaded upstream's static
    `ttyd.x86_64` release binary and verified it against `SHA256SUMS`
    instead) against the disposable VM `192.168.122.245`, not the user's
    real vault - a fresh scratch `$HOME` with its own vault, a server named
    "Office VPN", and a peer named "laptop" added live during recording.
    The demo does show that VM's real private IP on screen (SSH config
    alias resolution turned out to require a *real* passwd-registered
    `$HOME`, not an env-var override - OpenSSH silently ignores
    `~/.ssh/config` otherwise - so the alias approach that would have hidden
    it didn't pan out); accepted deliberately since a `192.168.x.x` address
    is private/non-routable and identifies nothing. The source `.tape`
    script is committed alongside it (`docs/assets/demo.tape`,
    parameterized via env vars) so the demo can be re-recorded after future
    UI changes.
  - **Correction after user review:** the first recording registered the
    demo server under the real `skorin` SSH login - the username is shown
    unmasked on screen (unlike the password, which huh always masks), so
    that was a real personal-data leak into a file meant for public
    distribution. Fixed by creating a dedicated low-privilege `demo` user
    on the disposable VM (`useradd -m demo`, sudo group membership,
    password `demo-password-123`) and re-recording under that account
    instead - `docs/assets/demo.gif` and the `.tape` script are both
    updated, and the `.tape` script's header now explicitly warns future
    re-recordings to use a dedicated account, never the operator's own
    login. Also cleared the VM's leftover raw WireGuard peers from earlier
    testing and re-added a single clean "laptop" peer so the demo's peer
    list isn't cluttered with unmanaged entries.
  - The repo is currently **private** - the new README is written as if
    public-facing, but nobody has changed the actual GitHub visibility
    setting; that's a separate, deliberate decision for the user to make,
    not something to flip unilaterally.

## Completed in the `v0.5.3` release cycle

- Fixed six more real bugs reported from actual Konsole/KDE usage:
  1. `updateServerList`'s raw key switch (`internal/tui/model.go`) still had
     its own untouched `case "p"` from before the `v0.5.1` rename to `W` -
     the visible accelerator changed but this independent binding, reached
     whenever no commander accelerator claims the keypress first, did not.
     Now `case "w"` only; `dispatchCommanderAction`'s internal synthesis
     updated to match. Regression: `TestPlainPNoLongerOpensPasswordForm`.
  2. Real rendering bug in `viewCommanderMenu`: the dropdown's horizontal
     offset was applied by prepending spaces to the *whole* multi-line panel
     string, which only indents its first line (the top border) - every
     other line, including every item row and the bottom border, stayed at
     column 0. Invisible for category 0 (offset 0, "only the first one
     displays correctly" is exactly what was reported), visibly broken for
     every other category, and this same mismatch made mouse.go's item
     hit-test (which assumes the render used the same offset) miss every row
     below the border. Fixed by indenting every line; confirmed both by a
     new regression test verifying every bordered line shares one column
     (`TestDropdownPanelIndentsEveryLineNotJustTheFirst`, which fails
     correctly against the old code) and by live tmux inspection.
  3. Server list and peer table rows are now mouse-clickable (select, like
     Up/Down - O/Enter still opens). Peer table clicks are intentionally
     scoped to "every peer fits in the table's visible height at once" -
     `bubbles/table@v1.0.0` exposes no public way to read its internal
     scroll offset from outside the package, so a click while scrolled
     would risk selecting the wrong peer; left a no-op instead. A follow-up
     bug in the first cut of the server-list click (missing an X-axis bound,
     so clicking the details panel in two-panel layout could be misread as
     a row click) was caught by its own regression test
     (`TestServerListRowClickIgnoresTheDetailsPanelInWideLayout`) before
     shipping - the bound needed to match `layoutModeFor(...) != layoutCompact`,
     not `== layoutWide`, since `viewServerList`'s two-panel layout covers
     both standard and wide.
  4. Root cause of "wrong sudo password loses its own explanation, shows a
     bare exit code / silently reports no privileges": `runInteractive`
     (`internal/sshtransport/client.go`) always returned only the *tail*
     of its captured output (after the last prompt it answered), but a
     rejected sudo password's "Sorry, try again." explanation is exactly
     the text that tail-stripping discards - and if the remote side closes
     the connection right after giving up rather than printing one more
     distinct summary line, nothing useful was left at all. `runInteractive`
     now returns the *entire* captured stream on failure (still just the
     clean tail on success). `RunSudo`/`RunSudoScript` - used by install,
     configure and peer operations, not only the initial connect that
     `v0.5.1` already covered - now detect and wrap a rejected password the
     same way `SudoValidate` does. `internal/tui/flow.go`'s
     `handleActionDone` clears the cached sudo password on any action that
     hits this, not just the initial discovery path.
  5. Documented connecting Claude Desktop/Claude Code to `wgtui mcp` with an
     actual config JSON example, and an explicit, honest note that baking
     the vault master password into a persistent client config file trades
     away wgtui's normal memory-only handling of it - a real trade-off, not
     just a formality.
- Live-verified end to end against `192.168.122.245` in tmux: intentionally
  entered a wrong sudo password and confirmed the new "sudo password was
  rejected by the server" message appears (both inline on Overview and as
  the status banner), that `R` (retry) re-prompts instead of silently
  reusing the rejected value, and that entering the correct password
  afterward recovers normally. Also confirmed live: the dropdown for a
  non-first category renders as a single aligned box, and a raw SGR mouse
  click correctly selects a different server-list row.
- `gofmt`, `go build`, `go vet`, `staticcheck v0.7.0`, `govulncheck v1.6.0`
  (0 called vulnerabilities), `go test ./...`, `go test -race ./...`,
  `CGO_ENABLED=0` Darwin amd64/arm64 cross-builds, and `goreleaser check`
  all pass on the changed tree.
- Pushed `master` directly (commit `93a910e`); the pre-push Semgrep gate (101
  rules, 114 files) was clean.
- Annotated tag `v0.5.3` points at `93a910e` and is pushed to origin. Release
  workflow run `31285544450` passed all three jobs (`semgrep`, `validate`,
  `goreleaser`).
- GitHub Release `v0.5.3` is published (not draft/prerelease) with
  `checksums.txt`, four Linux/macOS archives, two `.deb` files and two `.rpm`
  files.
- `scripts/install.sh v0.5.3` verified the published Linux amd64 archive and
  installed `wgtui 0.5.3` at `~/.local/bin/wgtui`.
- Installed `v0.5.3` binary SHA-256:
  `f94e106646f3e8d0570c7c87badba98040565d9bb760a8143446cfcfd1aa8844`.

## Completed in the `v0.5.2` release cycle

- Fixed the real root cause behind "top-menu hotkeys don't respond": with a
  dropdown already open (from an earlier `M` press or `Alt+S`), pressing a
  *different* category's `Alt+letter` fell through every case in
  `updateCommanderMenu` and did nothing - not a terminal/environment issue,
  a genuine gap in that function. `internal/tui/commander.go`'s
  `updateCommanderMenu` now checks `alt+<category key>` first and switches
  categories directly, matching what `handleCommanderAccelerator` already
  does from the closed state. Regression test:
  `TestAltCategorySwitchesWhileMenuAlreadyOpen`
  (`internal/tui/menu_redesign_test.go`).
- `gofmt`, `go build`, `go vet`, `staticcheck`, `go test ./...`,
  `go test -race ./...` pass on the changed tree.
- Pushed `master` directly (commit `5a432bf`); the pre-push Semgrep gate (101
  rules, 114 files) was clean.
- Annotated tag `v0.5.2` points at `5a432bf` and is pushed to origin. Release
  workflow run `31283381907` passed all three jobs (`semgrep`, `validate`,
  `goreleaser`).
- GitHub Release `v0.5.2` is published (not draft/prerelease) with
  `checksums.txt`, four Linux/macOS archives, two `.deb` files and two `.rpm`
  files.
- `scripts/install.sh v0.5.2` verified the published Linux amd64 archive and
  installed `wgtui 0.5.2` at `~/.local/bin/wgtui`.
- Installed `v0.5.2` binary SHA-256:
  `a1f2e390470f851695e38e995f73042d1cef8658b2e527f3997ba13498348186`.

## Completed in the `v0.5.1` release cycle

- Fixed six user-reported issues against the freshly released `v0.5.0` UI, found by
  testing the real installed binary against a KDE desktop terminal and the disposable
  VM `192.168.122.245`:
  1. Overview/Peers/Diagnose tabs are now mouse-clickable (`internal/tui/mouse.go`,
     `internal/tui/view.go`'s new `tabClickTarget`/`detailTabsRow`/`detailTabsColumn`,
     shared with the renderer so a click always lands on the tab it appears over).
  2. Fixed a real ANSI-corruption bug in the wide Diagnose panel: the selected row
     truncated already-styled badge text with a helper meant for plain user text,
     which strips the ESC byte out of embedded color codes and leaves the rest of
     each escape sequence as visible garbage. Added `checkStatusPlain` and rebuilt
     the selected row from plain text styled exactly once. Verified live via raw
     ANSI capture in tmux, not just unit tests.
  3. The vault password accelerator moved from `P` to `W` (pass**W**ord) so it can
     never be confused with `Alt+P` (Peer menu) - confirmed via screenshots that the
     confusion was real in practice. Added `TestBottomBarNeverReusesATopCategoryLetter`
     to keep this true going forward.
  4. SSH/sudo passwords are now cached for the lifetime of an open server session
     (`internal/tui/credentials.go`'s `credentialCache`, held on `*model.creds`,
     cleared on `returnToServerList` and specifically on a rejected sudo password)
     instead of once per individual SSH dial - install, post-install rediscovery and
     configure no longer re-prompt for the same secret. Deliberately stores `[]byte`,
     not `string`, so a clear can zero it in place; documented the residual
     ptrace/core-dump exposure this does not and cannot eliminate in pure Go without
     cgo/mlock, matching `docs/THREAT_MODEL.md`'s existing framing.
  5. A wrong sudo password now produces an explicit, distinguishable error
     (`sshtransport.ErrSudoPasswordRejected`, detected from sudo's own "incorrect
     password"/"Sorry, try again" wording, plumbed through
     `discovery.Report.System.SudoPasswordRejected`) instead of silently reporting
     generic "insufficient privileges" with no explanation.
  6. Installing WireGuard on a server with no interface now continues straight into
     the configure wizard (`model.afterInstall`) instead of stopping at Overview -
     matches SPEC.md §12.1's originally documented option 2 ("Установить и запустить
     мастер настройки сервера"), which the TUI had never actually wired up.
  7. Documented the already-working `wgtui mcp` front end in the README; it was
     real and functional (confirmed via a manual stdio `initialize` smoke test) but
     had no discoverable usage instructions outside `docs/ARCHITECTURE.md`.
- Added regression tests for all of the above:
  `internal/tui/menu_redesign_test.go` (bottom bar vs. top category letters),
  `internal/tui/credentials_test.go` (cache reuse/clear, rejected-sudo wiring),
  `internal/tui/server_actions_test.go` (`afterInstall` chaining),
  `internal/sshtransport/client_test.go` (`sudoPasswordWasRejected`).
- `gofmt`, `go build`, `go vet`, `staticcheck v0.7.0`, `govulncheck v1.6.0` (0 called
  vulnerabilities), `go test ./...`, `go test -race ./...`, `CGO_ENABLED=0` Darwin
  amd64/arm64 cross-builds, and `goreleaser check` all pass on the changed tree.
- Live-verified in tmux against an isolated `HOME` and the real `192.168.122.245`
  VM: bottom bar shows `W` highlighted inside "Password" with correct ANSI: the
  diagnose selected-row fix, no repeated SSH/sudo prompts across a Refresh, and the
  Overview/Diagnose screens render correctly end to end.
- Pushed `master` directly (commit `ec87360`); the pre-push Semgrep gate (101 rules,
  114 files) was clean.
- Annotated tag `v0.5.1` points at `ec87360` and is pushed to origin. Release workflow
  run `31283108813` passed all three jobs (`semgrep`, `validate`, `goreleaser`).
- GitHub Release `v0.5.1` is published (not draft/prerelease) with `checksums.txt`,
  four Linux/macOS archives, two `.deb` files and two `.rpm` files.
- `scripts/install.sh v0.5.1` verified the published Linux amd64 archive and
  installed `wgtui 0.5.1` at `~/.local/bin/wgtui`.
- Installed `v0.5.1` binary SHA-256:
  `7e181ea2e06cf6b88c516a015e0af9f9ca8d9d6b42f379dcb48a4628d02dd074`.
- **Found one more real bug while verifying `v0.5.1`'s own fixes**: with a
  top-menu dropdown already open (e.g. from an earlier `M` press or `Alt+S`),
  pressing a *different* category's `Alt+letter` fell through every case in
  `updateCommanderMenu` unmatched and did nothing - indistinguishable from "the
  hotkey doesn't respond", which is almost certainly what was originally
  reported. Fixed in `internal/tui/commander.go`'s `updateCommanderMenu` (checks
  `alt+<category key>` before anything else) with a regression test,
  `TestAltCategorySwitchesWhileMenuAlreadyOpen`. This fix was not yet in `v0.5.1`
  when it was tagged; see the `v0.5.2` cycle below.

## Completed in the `v0.5.0` release cycle

- Implemented `docs/UI_MENU_REDESIGN_HANDOFF.md` in full: the top menu
  (`Alt+letter`) and the bottom action bar (plain letter) are now two
  explicitly separate accelerator systems that never promise the same key
  twice. Removed the `│` separators from the top row; its active/dark
  background now shows only while a dropdown is actually open, never merely
  because a related screen is showing. Bottom-bar accelerator letters are
  highlighted inside their own label (`Remove`, `revOke`) instead of a
  separate key cell; `Remove` moved from `X` to `R` and `Revoke` from `X` to
  `O` so both are letters that actually appear in their labels. Added a
  shared `renderAcceleratedLabel` renderer and shared width-geometry helpers
  so the mouse hit-test can never disagree with what was rendered.
- Added acceptance tests for the redesign contract (`internal/tui/menu_redesign_test.go`):
  no `│` at any width, active background only while the matching dropdown is
  open, every non-symbol accelerator letter is present inside its own label,
  per-panel accelerator uniqueness, and mouse click / keyboard accelerator
  dispatching the same action.
- Verified the new chrome live in a tmux session at `64×18` and `100×30`
  against an isolated `HOME`, including inspecting raw ANSI output to confirm
  the active category actually renders with the dark background and calm
  categories render with the cyan one.
- Ran the full missing-WireGuard install flow for real (not read-only) against
  the disposable VM `192.168.122.245` (Ubuntu 24.04) through the actual TUI:
  discovery correctly classified `wireguard_missing`, the confirmation screen
  showed the exact `apt-get update` / `apt-get install -y wireguard` plan,
  applying it after the sudo-password prompt succeeded, `VerifyInstalled`
  passed, and the server reclassified to `tools_no_interfaces` with the
  bottom bar switching from `Install` to `Configure`. Independently confirmed
  over a second SSH session that `wireguard`/`wireguard-tools` are installed
  via `dpkg -l` and `wg --version`. The VM now has WireGuard installed but no
  interface configured; configuring an interface was out of this task's scope.
- `go build`, `go vet`, `staticcheck v0.7.0`, `govulncheck v1.6.0` (0 called
  vulnerabilities), `go test ./...` and `go test -race ./...`,
  `CGO_ENABLED=0` Darwin amd64/arm64 cross-builds, and `goreleaser check` all
  pass on the changed tree.
- Pushed `master` directly (commits `5fc41a9`, `4d3de40`); the pre-push hook's
  Semgrep scan (101 rules, 114 files) was clean both times.
- Annotated tag `v0.5.0` points at `4d3de40` and is pushed to origin. Release
  workflow run `31280917291` passed all three jobs (`semgrep`, `validate`,
  `goreleaser`).
- GitHub Release `v0.5.0` is published (not draft/prerelease) with
  `checksums.txt`, four Linux/macOS archives, two `.deb` files and two `.rpm`
  files.
- `scripts/install.sh v0.5.0` verified the published Linux amd64 archive and
  installed `wgtui 0.5.0` at `~/.local/bin/wgtui` with mode `0755`.
- Installed `v0.5.0` binary SHA-256:
  `e859b366639c36948b26a893a18554922a8f0d4a8c8c97a1e87152b80abeb6c4`.

## Completed in the `v0.4.0` release cycle

- Reviewed `Screenshot_20260808_180214.png` and the matching Midnight
  Commander screenshot. Confirmed that the top and bottom chrome needed solid
  contrast, explicit separators, a compact dropdown and real mouse hit areas.
- Replaced the visible F-key strip with contextual btop-style highlighted
  letter accelerators. F-keys remain undocumented compatibility aliases only.
- Enabled Bubble Tea cell-motion mouse support and added click dispatch for top
  categories, dropdown items and bottom actions; disabled actions remain safe.
- Moved initial vault create/unlock into a branded in-app password form.
- Replaced terminal release/read-password flows with in-app SSH host-key, SSH
  password and sudo password modals. Secrets are cleared from model fields and
  cached only inside the active remote session.
- Added access, credential, mouse, disabled-click, exact-width and minimum-size
  regressions.
- Anchored the final action row to the terminal's physical bottom row. Status
  banners no longer shift the visible bar away from its mouse hit area.
- Letter accelerators now honor the same readiness rules as disabled menu and
  mouse actions; they cannot bypass missing WireGuard/interface prerequisites.
- Verified the disposable VM `192.168.122.245` is Ubuntu 24.04 with systemd
  and no `wg` binary. The new read-only manual integration test passed full
  discovery, sudo validation, missing-WireGuard classification, diagnostics,
  distro detection and install-plan generation without mutating the server.

- Published and locally installed `v0.2.2`; the release contains Darwin and
  Linux archives for amd64/arm64 plus Linux `.deb`/`.rpm` packages.
- Installed `v0.2.2` binary SHA-256:
  `31a31434fd01f8ef91b1d4c9b3573dd66321457f42285b0da8ac56c456acbf8a`.
- User approved a pure-Go Commander UI direction inspired by Midnight
  Commander; S-Lang/CGO migration is explicitly out of scope to preserve macOS
  compatibility and the tested Go business layer.
- Added `docs/COMMANDER_UI.md` as the menu, action-bar, two-pane and cross-platform
  behavior contract for `v0.3.0`.
- Implemented the persistent Server/Peer/View/Tools/Help menu with disabled
  reasons, mouse/letter navigation and safe dispatch through existing workflows.
- The former F-key strip was superseded in `v0.4.0` by a contextual segmented
  letter-action bar whose rendered width is exact at every supported width from
  64 through 300 columns.
- Overview, Peers and Diagnose now retain stable collection/detail panes in
  wide mode; compact peer actions fit the supported `64×18` minimum.
- CI and release validation now cross-build both Darwin architectures with
  `CGO_ENABLED=0` before packaging.
- PR #13 passed CI #28 and merged into `master` as `4828162`.
- Annotated tag `v0.3.0` points at `4828162`; release workflow #6 completed
  Semgrep, validation and GoReleaser successfully.
- GitHub Release `v0.3.0` is published with checksums, four Linux/macOS
  archives, two `.deb` files and two `.rpm` files.
- `scripts/install.sh v0.3.0` verified the published Linux amd64 archive and
  installed the static `wgtui 0.3.0` binary at `~/.local/bin/wgtui`.
- Installed `v0.3.0` binary SHA-256:
  `bf0d425bbe4779e4ebaf0e3d30fb9c66a68431af3a860d8301442daf35476f52`.

- Reviewed the new `3063×1335` screenshot and reproduced its effective
  wide-layout sizes in the text renderer.
- Removed nested ANSI formatting from peer table cells so the parent table can
  render one continuous selected-row background and repaint safely.
- Added a semantic peer summary and the running version to the application
  header, making both state and screenshot provenance immediately visible.
- Added a wide-layout invariant that checks fixed positions for all four panel
  borders at every width `135..260`.

- Fixed stale asynchronous discovery/peer results crossing server or refresh
  boundaries by attaching server and request generation IDs.
- Peers opened during initial discovery wait for the session and load
  automatically instead of racing a missing connection.
- SSH exit 255 now retains diagnostics and triggers one bounded automatic
  reconnect; a second failure becomes a stable, actionable error.
- Fixed the reported broken cyan panel lines: table calculations now include
  renderer cell padding, so rows cannot overwrite the panel border.
- Long remote error messages are sanitized, wrapped and bounded inside the
  peer panel.

- Replaced the ambiguous post-peer-creation screen with an explicit client
  setup menu: QR, private profile+QR export, secret config text, or finish.
- Added four layout modes: below-minimum guard, compact, standard, and wide.
  Minimum is `64×18`; recommended size is `90×24`.
- Wide mode now requires adequate height as well as width. Short wide terminals
  fall back to the compact single-panel layout instead of overflowing.
- Shrinking a terminal or crossing a breakpoint requests a full renderer clear,
  preventing duplicated footers and stale borders.
- Added bounded diagnostic scrolling, deterministic interface ordering,
  many-interface summaries, QR height safety, compact help/overview screens,
  tiny-terminal fallback, and ANSI-safe status/footer truncation.
- Applied semantic colors across navigation, selection, connection/success,
  unmanaged/attention, error/danger, values, and secondary context. Every color
  retains a text label or symbol.
- Strengthened CI and release workflows with race tests, Staticcheck and
  govulncheck. Added `CHANGELOG.md` to release archives.
- Updated README, TUI and architecture documentation for the new contract.

## Test and packaging evidence

### Current `v0.4.0` branch

- `go test ./...`, `go test -race ./...`, `go vet ./...` and Staticcheck
  `v0.7.0`: pass.
- govulncheck `v1.6.0`: 0 called vulnerabilities and 0 vulnerabilities in
  imported packages.
- Semgrep: 101 rules on 112 repository files, 0 findings; a second explicit
  scan covered the four new security-relevant untracked Go files with 0
  findings.
- Darwin amd64 and arm64 `CGO_ENABLED=0` builds: valid Mach-O executables.
- Two 10-second fuzz runs: about 1.27M terminal-safety executions and 1.51M
  export-path executions, no failures.
- GoReleaser configuration check and clean snapshot: pass; Linux/macOS
  amd64/arm64 archives plus Linux deb/rpm packages and checksums produced.
- Read-only missing-WireGuard Ubuntu 24.04 VM integration: pass; no remote
  mutation was applied.
- PR #15 CI run `31269116803`: pass after stabilizing the vault guidance across
  runner-dependent text widths. Linux/macOS builds, vet, unit, race,
  Staticcheck, govulncheck and Semgrep are green at commit `98f3e0c`.
- PR #15 merged into `master` as `a63d3fb`; master CI run `31269480947`
  passed all build, macOS cross-build, vet, unit, race, Staticcheck,
  govulncheck and Semgrep gates.
- Annotated tag `v0.4.0` points at `a63d3fb` and is pushed to origin.
- Release workflow `31269636963` passed Semgrep, validation and GoReleaser.
- GitHub Release `v0.4.0` is published (not draft/prerelease) with checksums,
  four Linux/macOS archives, two `.deb` files and two `.rpm` files.
- `scripts/install.sh v0.4.0` verified the published Linux amd64 archive and
  installed `wgtui 0.4.0` at `~/.local/bin/wgtui` with mode `0755`.
- Installed `v0.4.0` binary SHA-256:
  `4a3f284fdf44955518a046a0ea3441726f2daa3827f53d795b82ed457d5556a3`.

- `go test ./...`: pass.
- `go test -race ./...`: pass.
- `go vet ./...`: pass.
- Staticcheck `v0.7.0`: pass after fixing all findings.
- govulncheck `v1.6.0`: 0 called/package vulnerabilities. It reports only the
  unmaintained `x/crypto/openpgp` package at module level; wgtui does not import
  or call it.
- Semgrep: 101 rules on 108 tracked files, 0 findings locally, in the
  pre-push hook, and in PR CI.
- Fuzzing: about 305k terminal-text/truncation inputs and 1.25M export-name
  inputs, no failures.
- Responsive property tests cover every width `64..180` over eight terminal
  heights, plus tiny sizes below the supported minimum.
- GoReleaser `v2.17.1` configuration check: pass.
- GoReleaser snapshot: pass for Linux/macOS amd64/arm64, producing four archives,
  four Linux packages and valid checksums. Archives contain the binary, license,
  README, changelog, man page and three shell completions.
- actionlint `v1.7.12`: pass for CI and release workflows.
- New peer-loading and rendering regressions pass, including every wide
  terminal width `135..260` and the reported screenshot-equivalent width.
- The affected TUI and SSH transport packages passed 20 consecutive test runs.
- Two additional 10-second fuzz runs completed about 390k terminal-text and
  662k export-path executions without failure.
- Commander UI verification passed: all package and race tests, vet,
  Staticcheck, Semgrep, govulncheck (0 called vulnerabilities), Darwin
  amd64/arm64 cross-builds, GoReleaser check/snapshot, about 596k terminal-text
  fuzz executions and 806k export-path fuzz executions.
- PR #10 passed build, vet, unit, race, Staticcheck, govulncheck and Semgrep CI,
  then merged into `master` as `0665b82`.
- Master CI #23 passed.
- Annotated tag `v0.2.1` points at `0665b82` and is pushed to origin.
- Release workflow #4 passed Semgrep, validation and GoReleaser.
- GitHub Release `v0.2.1` is published (not draft/prerelease) with checksums,
  four Linux/macOS archives, two `.deb` files and two `.rpm` files.
- `scripts/install.sh v0.2.1` verified the published archive checksum and
  installed `wgtui 0.2.1` at `~/.local/bin/wgtui` with mode `0755`.
- Installed `v0.2.1` binary SHA-256:
  `8f04b163e3ba25277fd8d8e354ad863116c9fcd55f6958f942292bca6ac48e2d`.
- PR #7 (`Release v0.2.0: polished responsive TUI`) passed both GitHub CI
  jobs, including build, vet, unit, race, Staticcheck, govulncheck and Semgrep.
- PR #7 was merged into `master` as `682924c`.
- Final checkpoint PR #8 passed CI and was merged as `f337526`.
- Annotated tag `v0.2.0` points at `f337526` and is pushed to origin.
- Master CI #19 passed.
- Release workflow #3 passed Semgrep, build, vet, race, Staticcheck,
  govulncheck and GoReleaser.
- GitHub Release `v0.2.0` is published (not draft/prerelease) with checksums,
  four Linux/macOS archives, two `.deb` files and two `.rpm` files.
- `scripts/install.sh v0.2.0` verified the published archive checksum and
  installed `wgtui 0.2.0` at `~/.local/bin/wgtui` with mode `0755`.
- Installed binary SHA-256:
  `ecb9853a17ed28beebca3f10d67df8c8b11d3d1be1467b77cb8426026bb2e61a`.

## Resume here

`v0.5.7` is complete, published and installed; there is no pending release
work. Future product work should start from a clean synchronized `master`.

`192.168.122.245` (disposable Ubuntu 24.04 test VM) current state: has a
dedicated `demo` user (password `demo-password-123`, sudo group member) -
created specifically so demo recordings never show the operator's real
login name on screen (see the `v0.5.4`→documentation-pass entry below for
why). WireGuard is installed and configured on `wg0` with one peer
("laptop", added through wgtui, matching what `docs/assets/demo.gif`
shows). Don't repeat the install/configure test from scratch unless
install/discovery behavior actually changes; do feel free to reuse the
`demo` account for anything else that needs a non-identifying SSH user on
this VM (another demo recording, MCP verification, etc.) instead of the
original `skorin` login.

The user tests the live installed binary hands-on (Konsole/KDE) and has
reported real, reproducible bugs across every release cycle so far
(`v0.5.0`→`v0.5.1`, a bug found while verifying `v0.5.1` itself→`v0.5.2`, a
further round of Konsole-specific reports→`v0.5.3`, a header/scroll/ordering
round→`v0.5.4`, a real MCP schema bug caught by actually trying the
README's own MCP walkthrough→`v0.5.5`, a second, missed instance of a bug
I'd already claimed fixed elsewhere→`v0.5.6`, and a screenshot proving the
*actual* ask - move the brand line to the bottom, not just deduplicate it
at the top - still hadn't been done anywhere→`v0.5.7`). Every single one
turned out to be a genuine gap, never a terminal/environment quirk or user
error - take reports like this at face value and look for a real cause
first. Recurring patterns worth checking early:

- **"Hotkey doesn't respond" / visual misalignment / "mouse doesn't work
  here":** does a *raw*/independent key handler (`updateServerList`,
  `updateServerDetail`, etc.) still bind something the commander-accelerator
  layer was supposed to have moved or renamed? Does a renderer building a
  multi-line block apply an offset to the whole string (only the first line
  actually moves) instead of to every line?
- **Anything MCP-related:** verify with the real `claude` CLI
  (`claude mcp add`/`claude mcp list`/actually asking Claude to use a tool),
  not only a Go-level unit test against the SDK's own in-memory client -
  those exercise different code paths, and the schema-validation bug fixed
  in `v0.5.5` only ever showed up in the former. This was a direct, fair
  correction from the user after an earlier claim of "verified" turned out
  to mean "the unit test passes."
- **Before reporting any UI bug "fixed," grep for every other call site of
  the same pattern, not just the one screen the report named.** The
  `v0.5.6` bug existed because the `v0.5.4` header-clutter fix was applied
  correctly to the server-detail screen but the identical
  "chrome-bar-then-`appHeader()`" shape was never checked for on the
  vault-access or credential-prompt screens - it was there the whole time,
  just unreported because those screens weren't the one originally flagged.
  A fix scoped to "the screen the user mentioned" is not the same claim as
  "the bug is fixed" - say which one you actually verified.
- **When a fix is described in words ("move X down"), don't assume the
  first plausible interpretation is the intended one - ask what "down"
  concretely means, or wait for a screenshot, before declaring it done.**
  `v0.5.4` read "move the version signature down" as "off the crowded top
  row, one row lower" and shipped that; the user's `v0.5.7` screenshot
  (red arrow, top of screen to bottom) showed the actual ask was "next to
  the bottom action bar." A verbal description of a layout change is
  genuinely ambiguous - when in doubt, this project's convention is now to
  treat "moved down" as "moved to sit just above the bottom key/action
  bar" unless told otherwise.

## Publishing note

`git push` works over the configured SSH remote. GitHub CLI is authenticated and
the connected GitHub app works for PR and merge operations.

## Useful commands

```bash
cd /home/skorin/Git/wgtui
git status --short --branch
git log -5 --oneline
go test ./...
go test -race ./...
go vet ./...
~/.local/bin/goreleaser check
```
