package cli

import (
	"fmt"
	"io"
)

// runCompletion implements `wgtui completion <shell>` (spec §28, Stage 5).
// wgtui's command surface is small and hand-rolled on the standard `flag`
// package rather than a framework with generated completion (e.g. cobra),
// so the scripts below are static: they complete the fixed command/
// subcommand tree in root.go's usage text, not per-server or per-peer
// names (which only the vault, not the shell, knows about).
func runCompletion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "wgtui completion: expected exactly one shell argument: bash, zsh, or fish")
		return 2
	}
	switch args[0] {
	case "bash":
		fmt.Fprint(stdout, bashCompletion)
	case "zsh":
		fmt.Fprint(stdout, zshCompletion)
	case "fish":
		fmt.Fprint(stdout, fishCompletion)
	default:
		fmt.Fprintf(stderr, "wgtui completion: unknown shell %q, expected bash, zsh, or fish\n", args[0])
		return 2
	}
	return 0
}

const bashCompletion = `# wgtui bash completion
# Install: wgtui completion bash > /etc/bash_completion.d/wgtui
# Or for the current user only: wgtui completion bash >> ~/.bash_completion
_wgtui_completions() {
	local cur prev words cword
	_init_completion || return

	local top="server peer diagnose mcp completion version help"
	local server_sub="add inspect install-wireguard configure"
	local peer_sub="list add disable enable revoke rotate export import"
	local shells="bash zsh fish"

	if [[ $cword -eq 1 ]]; then
		COMPREPLY=($(compgen -W "$top" -- "$cur"))
		return
	fi

	case "${words[1]}" in
	server)
		if [[ $cword -eq 2 ]]; then
			COMPREPLY=($(compgen -W "$server_sub" -- "$cur"))
		fi
		;;
	peer)
		if [[ $cword -eq 2 ]]; then
			COMPREPLY=($(compgen -W "$peer_sub" -- "$cur"))
		fi
		;;
	completion)
		if [[ $cword -eq 2 ]]; then
			COMPREPLY=($(compgen -W "$shells" -- "$cur"))
		fi
		;;
	esac
}
complete -F _wgtui_completions wgtui
`

const zshCompletion = `#compdef wgtui
# wgtui zsh completion
# Install: wgtui completion zsh > "${fpath[1]}/_wgtui"

_wgtui() {
	local -a top server_sub peer_sub shells
	top=(server peer diagnose mcp completion version help)
	server_sub=(add inspect install-wireguard configure)
	peer_sub=(list add disable enable revoke rotate export import)
	shells=(bash zsh fish)

	if (( CURRENT == 2 )); then
		compadd -a top
		return
	fi

	case "${words[2]}" in
	server)
		(( CURRENT == 3 )) && compadd -a server_sub
		;;
	peer)
		(( CURRENT == 3 )) && compadd -a peer_sub
		;;
	completion)
		(( CURRENT == 3 )) && compadd -a shells
		;;
	esac
}
_wgtui
`

const fishCompletion = `# wgtui fish completion
# Install: wgtui completion fish > ~/.config/fish/completions/wgtui.fish
complete -c wgtui -f

set -l top server peer diagnose mcp completion version help
set -l server_sub add inspect install-wireguard configure
set -l peer_sub list add disable enable revoke rotate export import
set -l shells bash zsh fish

complete -c wgtui -n "not __fish_seen_subcommand_from $top" -a "$top"
complete -c wgtui -n "__fish_seen_subcommand_from server; and not __fish_seen_subcommand_from $server_sub" -a "$server_sub"
complete -c wgtui -n "__fish_seen_subcommand_from peer; and not __fish_seen_subcommand_from $peer_sub" -a "$peer_sub"
complete -c wgtui -n "__fish_seen_subcommand_from completion; and not __fish_seen_subcommand_from $shells" -a "$shells"
`
