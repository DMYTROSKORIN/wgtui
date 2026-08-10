# Regenerates docs/assets/demo.gif (README's "See it" section).
#
# Requires: vhs (https://github.com/charmbracelet/vhs), ttyd, and a Chrome/
# Chromium binary on PATH - vhs drives a real ttyd-served terminal through a
# headless browser to capture frames.
#
#   go install github.com/charmbracelet/vhs@latest
#
# This tape assumes $DEMO_HOME already has a vault (master password
# "demopassword123") with one server registered ("Office VPN", reachable,
# WireGuard installed, at least one peer) - it only unlocks and browses, it
# doesn't build that state from scratch, so a real (disposable/test) server
# is still needed once to set that up by hand first. Never point this at a
# vault holding real production servers - the recording shows real key facts
# on screen (server name, OS, interface state, SSH username), even though it
# never shows a real password (huh's password fields mask every character).
#
# Register the demo server under a dedicated low-privilege account created
# just for this (e.g. "demo", `useradd -m demo && usermod -aG sudo demo`),
# not your own real login user - the SSH username *is* shown on screen
# unmasked, and a real personal username ending up in a published recording
# is exactly the mistake this note exists to prevent (it happened once).
#
# Run from the repo root:
#   DEMO_HOME=/path/to/a/throwaway/HOME \
#   DEMO_VAULT_PASSWORD='demopassword123' \
#   DEMO_SSH_PASSWORD='...' \
#     vhs docs/assets/demo.tape
#
# The Sleep/TypingSpeed values here are deliberately generous (confirmed
# needed on a fresh vhs+ttyd+headless-Chrome setup, which otherwise
# silently dropped characters mid-password and desynced the whole
# recording - Esc later in the script then quit the app early because the
# vault was never actually unlocked). If vhs still proves unreliable in a
# given environment, the same flow can be driven directly instead:
# `asciinema rec --command 'HOME=$DEMO_HOME wgtui' out.cast` in one
# terminal, drive it with real keystrokes (or `tmux send-keys` against the
# session running that command) following this file's steps, then
# `agg --font-size 18 --theme dracula --speed 2.2 --idle-time-limit 2
# out.cast docs/assets/demo.gif`.

Output "docs/assets/demo.gif"

Set Shell "bash"
Set FontSize 18
Set Width 1120
Set Height 760
Set Padding 20
Set WindowBar Colorful
Set Theme "Dracula"
Set TypingSpeed 100ms

Hide
Type "clear && HOME=$DEMO_HOME wgtui"
Enter
Sleep 2s
Show

# Unlock the existing local vault
Sleep 1500ms
Type "$DEMO_VAULT_PASSWORD" Sleep 500ms
Enter
Sleep 1s

# Server list (already has one server registered)
Sleep 1000ms
Enter
Sleep 2s

# SSH + sudo prompts
Sleep 500ms
Type "$DEMO_SSH_PASSWORD" Sleep 500ms
Enter
Sleep 2s
Sleep 500ms
Type "$DEMO_SSH_PASSWORD" Sleep 500ms
Enter
Sleep 3500ms

# Overview tab
Sleep 2s

# Menu ("m" opens on View from Overview) -> Peers
Type "m"
Sleep 1200ms
Type "p"
Sleep 2500ms

# Reopen the menu and browse categories
Type "m"
Sleep 1000ms
Right
Sleep 600ms
Right
Sleep 600ms
Escape
Sleep 1s

# Diagnose tab
Type "d"
Sleep 2500ms

Sleep 2s
Ctrl+c
Sleep 500ms
