#!/usr/bin/env bash
# Regenerates docs/assets/demo.gif (the README hero recording).
#
# Requires: tmux, asciinema (3.x), agg (https://github.com/asciinema/agg),
# gifsicle. Earlier versions of this recording were made with vhs; it
# dropped keystrokes mid-password on a fresh machine often enough that the
# asciinema+tmux route below replaced it for good.
#
# What the recording shows: unlock the local vault, open a registered
# server, answer the SSH and sudo prompts, Overview, Peers, add a peer
# through the form + review-plan + apply, the "peer is ready" screen,
# then Diagnose. The QR screen is deliberately left out: it is 39 rows
# tall, and a README hero image has to be wide, not a vertical strip -
# the earlier 112x65 recording was pulled for exactly that reason.
#
# Prerequisites this script does NOT set up (do it once, by hand):
#   - a throwaway $DEMO_HOME whose vault (master password $DEMO_VAULT_PASSWORD)
#     has one server registered ("Office VPN") that is reachable, has
#     WireGuard installed and configured, and ideally one existing peer.
#     The host's SSH key must already be in $DEMO_HOME/.ssh/known_hosts
#     (wgtui never relaxes host-key checking, so a first-connect prompt
#     would land in the recording).
#   - that server registered under a dedicated low-privilege account (e.g.
#     "demo"), never your real login - the SSH username is shown on screen
#     unmasked. Never point this at a vault holding production servers.
#   - a peer named $DEMO_PEER must not exist yet on the server (the run
#     really adds it; revoke it again afterwards or pick another name).
#
# Usage, from the repo root:
#   DEMO_HOME=/path/to/throwaway/home \
#   DEMO_VAULT_PASSWORD='...' DEMO_SSH_PASSWORD='...' \
#     docs/assets/record-demo.sh
#
# Timing notes learned the hard way:
#   - Every pause lives in this script, so the recording is driven in one
#     go. Driving it interactively from another shell records the gaps
#     between your own commands as idle time and bloats the cast.
#   - Don't resize the tmux pane mid-recording: agg renders everything
#     after a resize event broken, even though the cast data is fine.
#   - agg's --idle-time-limit caps the two or three long network waits
#     (connect+discovery, apply) so they read as a beat, not a stall.
set -euo pipefail

: "${DEMO_HOME:?set DEMO_HOME to a throwaway home directory with a prepared vault}"
: "${DEMO_VAULT_PASSWORD:?set DEMO_VAULT_PASSWORD}"
: "${DEMO_SSH_PASSWORD:?set DEMO_SSH_PASSWORD}"
DEMO_PEER=${DEMO_PEER:-phone}
COLS=${DEMO_COLS:-110}
ROWS=${DEMO_ROWS:-30}
OUT=${DEMO_OUT:-docs/assets/demo.gif}
WORK=$(mktemp -d)
CAST=$WORK/demo.cast
SESSION=wgtui-demo-rec
trap 'rm -rf "$WORK"' EXIT

k() { tmux send-keys -t "$SESSION" "$@"; }
# Human-paced typing for visible fields. Password fields are masked, so
# they are sent in one go - the pacing would be invisible anyway.
typeslow() {
	local s=$1 i
	for ((i = 0; i < ${#s}; i++)); do
		k -l "${s:i:1}"
		sleep 0.09
	done
}

tmux kill-session -t "$SESSION" 2>/dev/null || true
tmux new-session -d -s "$SESSION" -x "$COLS" -y "$ROWS" \
	"asciinema rec --overwrite -c 'env HOME=$DEMO_HOME TERM=xterm-256color wgtui' '$CAST'; sleep 3"
sleep 2.5

# Unlock the local vault
sleep 1.5
k -l "$DEMO_VAULT_PASSWORD"; sleep 0.6; k Enter
sleep 2.0

# Server list -> open the registered server
k Enter
sleep 2.0

# SSH password, then sudo password
k -l "$DEMO_SSH_PASSWORD"; sleep 0.6; k Enter
sleep 2.2
k -l "$DEMO_SSH_PASSWORD"; sleep 0.6; k Enter

# Connect + discovery, then hold on Overview
sleep 7.0

# Peers tab
k Tab
sleep 3.0

# New peer: name, tunnel mode, keepalive, DNS, endpoint, preshared key
k n
sleep 1.2
typeslow "$DEMO_PEER"; sleep 0.8; k Enter
sleep 1.0; k Enter
sleep 1.0; k Enter
sleep 0.8; k Enter
sleep 0.8; k Enter
sleep 1.0; k Enter
sleep 3.5 # let the review plan be read
k y
sleep 8.5 # applying -> "Peer is ready"
k Down; sleep 0.5; k Down; sleep 0.5; k Down; sleep 0.8
k Enter # "Finish for now" -> back to the Peers list
sleep 3.0

# Diagnose, then quit
k d
sleep 5.0
k q
sleep 2.0

for _ in $(seq 1 20); do
	tmux has-session -t "$SESSION" 2>/dev/null || break
	sleep 0.5
done
tmux kill-session -t "$SESSION" 2>/dev/null || true

if grep -q -F -e "$DEMO_VAULT_PASSWORD" -e "$DEMO_SSH_PASSWORD" "$CAST"; then
	echo "record-demo.sh: a password leaked into the cast - not rendering" >&2
	exit 1
fi

agg --font-size 14 --theme dracula --fps-cap 12 --idle-time-limit 4 "$CAST" "$WORK/raw.gif"
gifsicle -O3 --lossy=60 --colors 128 "$WORK/raw.gif" -o "$OUT"
gifsicle --info "$OUT" | head -2
ls -la "$OUT"
