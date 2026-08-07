# Project checkpoint

Last updated: 2026-08-07 22:31 CEST  
Active branch: `feat/tui-usability-overhaul`  
Last functional commit: `f7a64d2` (`adapt peer layout and export QR bundles`)

This file is the durable restart point for work on `wgtui`. After an interrupted
session, read it before changing code, then confirm the actual repository state
with `git status --short --branch` and `git log -5 --oneline`.

## Completed

- The TUI usability overhaul is implemented and pushed to the feature branch.
- The dashboard, vault-password settings, peer workflow, wide peer layout,
  default client DNS, secure portable export, and QR export bundle are in place.
- Documentation and automated tests were updated with the implementation.
- `go test ./...` passed on 2026-08-07.
- A stripped development build from `f7a64d2` was installed locally at
  `~/.local/bin/wgtui`, together with the man page and shell completions.
- Installed binary version: `0.0.0-dev`.
- Installed binary SHA-256:
  `80cbd96e93f3ae5ceb149fa329166ff38369bcf57dff5a0663feb9263e5089dd`.

## Resume here

The next task is a manual smoke test of the current full-screen TUI in a real
terminal against the already configured WireGuard server. Concentrate on:

1. dashboard layout at narrow and wide terminal sizes;
2. opening the peer list and moving between peers;
3. add-peer form defaults, especially DNS;
4. peer details and QR display/export bundle;
5. pause/resume/rotate/revoke confirmations and cancellation paths;
6. returning between screens and exiting with `Ctrl+C` without corrupting the
   terminal.

Do not repeat the completed redesign unless the smoke test finds a regression.
Record every discovered issue here or in a commit before ending the session.

## Remaining plan

- [ ] Run the manual TUI smoke test described above.
- [ ] Fix and test any regressions found during the smoke test.
- [ ] Run `go test ./...`, `go build ./...`, and `scripts/semgrep-scan.sh`.
- [ ] Push the final feature branch and verify its CI checks.
- [ ] Merge the feature branch into `master` when the smoke test and CI are clean.
- [ ] Create and locally install a tagged release if a release is wanted.

## Useful commands

```bash
cd /home/skorin/Git/wgtui
git status --short --branch
git log -5 --oneline
go test ./...
go build ./...
~/.local/bin/wgtui version
```
