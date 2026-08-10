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
# vault was never actually unlocked).
#
# In practice vhs has been unreliable enough on a fresh machine that the
# actually-published GIF was produced with a different toolchain instead -
# worth reaching for directly rather than fighting vhs again:
#
#   1. `asciinema rec --command 'HOME=$DEMO_HOME wgtui' out.cast` inside a
#      tmux pane sized to the FINAL dimensions you want (e.g.
#      `tmux new-session -x 112 -y 65 ...`). Do not resize the pane mid-
#      recording - agg's rendering of everything *after* a resize event
#      breaks (screens after it come out blank/overlapping) even though
#      the underlying cast data is fine; asciinema-cast player elsewhere
#      may not have this problem, but agg does. Pick a size tall enough
#      for the QR screen (`peer add` -> "Show QR code") up front instead.
#   2. Drive it with `tmux send-keys` against that session, following this
#      file's steps, with real pauses between actions (a couple of
#      seconds on each screen) so a viewer can actually read it - don't
#      try to make it snappy, that reads as confusing, not polished.
#   3. Tool round-trip latency between your own driving commands gets
#      recorded as real idle time on the cast, easily bloating a ~40s
#      demo to 200s+. Fix it by editing the .cast JSON lines directly
#      (each is `[delta_seconds, "o"|"r", data]`) rather than fighting it
#      live: cap ordinary deltas to something small (~0.05s), then assign
#      real hold durations only at specific content-matched moments - but
#      anchor each hold to the *last* event of that screen's paint burst
#      (chunks keep arriving in small pieces; a full repaint starts with
#      the `\x1b[H` cursor-home sequence), never the first chunk that
#      happens to contain the matched text - holding on an early chunk
#      freezes the GIF on a half-drawn/overlapping screen.
#   4. Keep `--font-size` modest (12, not 18) and the pane no taller than
#      it needs to be - a 112x65 pane at font-size 18 rendered a ~970KB,
#      824x1663px GIF for a README hero image, most of it just tall empty
#      panel padding below each screen's actual content. font-size 12 at
#      the same 112x65 cuts that roughly in half on its own.
#   5. `agg --font-size 12 --theme dracula --fps-cap 12 out.cast raw.gif`,
#      then `gifsicle -O3 --lossy=60 --colors 128 raw.gif -o
#      docs/assets/demo.gif` - lossy GIF compression doesn't noticeably
#      degrade terminal text or even the dense QR-code screen, and cut the
#      final file from ~970KB to ~270KB in practice.

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
