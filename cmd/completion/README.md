# Shell Auto-Completion for hyperctl

This package provides shell auto-completion support for the `hyperctl` CLI tool.

## Supported Shells

- **Bash** (4.0+)
- **Zsh** (5.0+)
- **Fish** (3.0+)

## Features

- Command completion (submit, query, list, export, info, grep, rg, vm, status, cancel, migrate, completion)
- `--provider` flag completion (`vsphere`, `nutanix`)
- Flag completion for each command
- Value completion for specific flags (e.g., `-op shutdown|poweroff|remove-cdrom|info`)
- File/directory completion where appropriate
- Context-aware completion based on command and previous flags

## Installation

### Bash

```bash
# For current session:
source <(hyperctl completion -shell bash)

# For all sessions (Linux):
hyperctl completion -shell bash | sudo tee /etc/bash_completion.d/hyperctl > /dev/null

# For all sessions (macOS):
hyperctl completion -shell bash > /usr/local/etc/bash_completion.d/hyperctl
```

### Zsh

```bash
# For current session:
source <(hyperctl completion -shell zsh)

# For all sessions:
hyperctl completion -shell zsh > "${fpath[1]}/_hyperctl"

# Rebuild completion cache:
rm -f ~/.zcompdump; compinit
```

### Fish

```bash
# For current session:
hyperctl completion -shell fish | source

# For all sessions:
hyperctl completion -shell fish > ~/.config/fish/completions/hyperctl.fish
```

## Usage Examples

Once installed, you can use tab completion with hyperctl:

```bash
# Complete commands
hyperctl <TAB>
# Shows: submit, query, list, grep, rg, vm, status, cancel, migrate, help, completion

# Complete flags for submit command
hyperctl submit -<TAB>
# Shows: -file, -vm, -output

# Complete operations for vm command
hyperctl vm -op <TAB>
# Shows: shutdown, poweroff, remove-cdrom, info

# Complete status values for query command
hyperctl query -status <TAB>
# Shows: running, completed, failed, cancelled, pending

# Complete provider for export
hyperctl export --provider <TAB>
# Shows: vsphere, nutanix

# Complete Nutanix VM list
hyperctl list --provider nutanix --server prism.example.com --user admin --pass 'xxx'
# Shows: name, path, os, power, all

# Complete color options for rg command
hyperctl rg -color <TAB>
# Shows: auto, always, never
```

## Development

### Adding New Commands

To add completion for a new command:

1. Add the command to the `commands` variable in each shell script
2. Add command-specific flags in the appropriate section
3. Add case handling in the argument completion section
4. Update tests in `completion_test.go`

### Testing

```bash
# Run all tests
go test ./cmd/completion/...

# Run specific test
go test ./cmd/completion/... -run TestBashCompletion

# Test generation
hyperctl completion -shell bash
hyperctl completion -shell zsh
hyperctl completion -shell fish
```

## Package Structure

- `completion.go` - Main completion logic and shell type definition
- `bash.go` - Bash completion script generator
- `zsh.go` - Zsh completion script generator
- `fish.go` - Fish completion script generator
- `completion_test.go` - Comprehensive test suite
- `README.md` - This file

## Technical Details

### Bash Completion

Uses the `complete -F` mechanism with the `_hyperctl_completion` function. Leverages bash-completion's `_init_completion` helper when available.

### Zsh Completion

Uses zsh's `_arguments` system with completion descriptions. Organized as a standard zsh completion function `_hyperctl`.

### Fish Completion

Uses fish's `complete` builtin with subcommand detection via `__fish_use_subcommand` and `__fish_seen_subcommand_from`.

## License

SPDX-License-Identifier: Apache-2.0
