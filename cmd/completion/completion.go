// SPDX-License-Identifier: Apache-2.0

// Package completion provides shell auto-completion support for hyperctl
package completion

import (
	"fmt"
)

// Shell represents a supported shell type
type Shell string

const (
	// ShellBash represents bash shell
	ShellBash Shell = "bash"
	// ShellZsh represents zsh shell
	ShellZsh Shell = "zsh"
	// ShellFish represents fish shell
	ShellFish Shell = "fish"
)

// Generate generates completion script for the specified shell
func Generate(shell Shell) (string, error) {
	switch shell {
	case ShellBash:
		return BashCompletion(), nil
	case ShellZsh:
		return ZshCompletion(), nil
	case ShellFish:
		return FishCompletion(), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
	}
}

// InstallInstructions returns installation instructions for the given shell
func InstallInstructions(shell Shell) string {
	switch shell {
	case ShellBash:
		return `
# To load completions in your current shell session:
source <(hyperctl completion -shell bash)

# To load completions for every session, execute once:
# Linux:
hyperctl completion -shell bash | sudo tee /etc/bash_completion.d/hyperctl > /dev/null

# macOS:
hyperctl completion -shell bash > /usr/local/etc/bash_completion.d/hyperctl
`
	case ShellZsh:
		return `
# To load completions in your current shell session:
source <(hyperctl completion -shell zsh)

# To load completions for every session, execute once:
hyperctl completion -shell zsh > "${fpath[1]}/_hyperctl"

# You may need to force rebuild of zcompdump:
rm -f ~/.zcompdump; compinit
`
	case ShellFish:
		return `
# To load completions in your current shell session:
hyperctl completion -shell fish | source

# To load completions for every session, execute once:
hyperctl completion -shell fish > ~/.config/fish/completions/hyperctl.fish
`
	default:
		return ""
	}
}

// SupportedShells returns a list of supported shells
func SupportedShells() []Shell {
	return []Shell{ShellBash, ShellZsh, ShellFish}
}
