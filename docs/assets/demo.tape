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

Output "docs/assets/demo.gif"

Set Shell "bash"
Set FontSize 18
Set Width 1120
Set Height 760
Set Padding 20
Set WindowBar Colorful
Set Theme "Dracula"
Set TypingSpeed 40ms

Hide
Type "clear && HOME=$DEMO_HOME wgtui"
Enter
Sleep 1s
Show

# Unlock the existing local vault
Sleep 800ms
Type "$DEMO_VAULT_PASSWORD" Sleep 300ms
Enter
Sleep 1s

# Server list (already has one server registered)
Sleep 1000ms
Enter
Sleep 2s

# SSH + sudo prompts
Type "$DEMO_SSH_PASSWORD" Sleep 300ms
Enter
Sleep 2s
Type "$DEMO_SSH_PASSWORD" Sleep 300ms
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
