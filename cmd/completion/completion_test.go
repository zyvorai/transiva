// SPDX-License-Identifier: Apache-2.0

package completion

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name    string
		shell   Shell
		wantErr bool
	}{
		{
			name:    "bash",
			shell:   ShellBash,
			wantErr: false,
		},
		{
			name:    "zsh",
			shell:   ShellZsh,
			wantErr: false,
		},
		{
			name:    "fish",
			shell:   ShellFish,
			wantErr: false,
		},
		{
			name:    "unsupported",
			shell:   Shell("powershell"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := Generate(tt.shell)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for unsupported shell")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if script == "" {
				t.Error("expected non-empty completion script")
			}
		})
	}
}

func TestBashCompletion(t *testing.T) {
	script := BashCompletion()

	// Check for required elements
	required := []string{
		"_hyperctl_completion",
		"complete -F _hyperctl_completion hyperctl",
		"submit",
		"query",
		"list",
		"grep",
		"rg",
		"vm",
		"status",
		"cancel",
		"migrate",
	}

	for _, req := range required {
		if !strings.Contains(script, req) {
			t.Errorf("bash completion missing required element: %s", req)
		}
	}

	// Check for command-specific flags
	if !strings.Contains(script, "submit_flags") {
		t.Error("bash completion missing submit_flags")
	}

	if !strings.Contains(script, "query_flags") {
		t.Error("bash completion missing query_flags")
	}
}

func TestZshCompletion(t *testing.T) {
	script := ZshCompletion()

	// Check for required elements
	required := []string{
		"#compdef hyperctl",
		"_hyperctl",
		"submit:Submit export job",
		"query:Query job status",
		"list:List VMs",
		"grep:Search VMs",
		"rg:Advanced search",
	}

	for _, req := range required {
		if !strings.Contains(script, req) {
			t.Errorf("zsh completion missing required element: %s", req)
		}
	}

	// Check for argument handling
	if !strings.Contains(script, "_arguments") {
		t.Error("zsh completion missing _arguments")
	}
}

func TestFishCompletion(t *testing.T) {
	script := FishCompletion()

	// Check for required elements
	required := []string{
		"complete -c hyperctl",
		"submit",
		"query",
		"list",
		"grep",
		"rg",
		"vm",
		"status",
		"cancel",
		"migrate",
	}

	for _, req := range required {
		if !strings.Contains(script, req) {
			t.Errorf("fish completion missing required element: %s", req)
		}
	}

	// Check for subcommand handling
	if !strings.Contains(script, "__fish_use_subcommand") {
		t.Error("fish completion missing __fish_use_subcommand")
	}

	if !strings.Contains(script, "__fish_seen_subcommand_from") {
		t.Error("fish completion missing __fish_seen_subcommand_from")
	}
}

func TestInstallInstructions(t *testing.T) {
	tests := []struct {
		name       string
		shell      Shell
		want       []string
		expectEmpty bool
	}{
		{
			name:  "bash",
			shell: ShellBash,
			want: []string{
				"source <(hyperctl completion",
				"/etc/bash_completion.d/hyperctl",
			},
		},
		{
			name:  "zsh",
			shell: ShellZsh,
			want: []string{
				"source <(hyperctl completion",
				"_hyperctl",
				"compinit",
			},
		},
		{
			name:  "fish",
			shell: ShellFish,
			want: []string{
				"hyperctl completion",
				"~/.config/fish/completions/hyperctl.fish",
			},
		},
		{
			name:        "unsupported shell",
			shell:       Shell("powershell"),
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instructions := InstallInstructions(tt.shell)

			if tt.expectEmpty {
				if instructions != "" {
					t.Error("expected empty instructions for unsupported shell")
				}
				return
			}

			if instructions == "" {
				t.Error("expected non-empty instructions")
			}

			for _, wantStr := range tt.want {
				if !strings.Contains(instructions, wantStr) {
					t.Errorf("instructions missing: %s", wantStr)
				}
			}
		})
	}
}

func TestSupportedShells(t *testing.T) {
	shells := SupportedShells()

	if len(shells) != 3 {
		t.Errorf("expected 3 supported shells, got %d", len(shells))
	}

	expectedShells := map[Shell]bool{
		ShellBash: false,
		ShellZsh:  false,
		ShellFish: false,
	}

	for _, shell := range shells {
		if _, exists := expectedShells[shell]; !exists {
			t.Errorf("unexpected shell: %s", shell)
		}
		expectedShells[shell] = true
	}

	for shell, found := range expectedShells {
		if !found {
			t.Errorf("missing expected shell: %s", shell)
		}
	}
}

func TestCompletionScriptSyntax(t *testing.T) {
	tests := []struct {
		name   string
		shell  Shell
		checks []string
	}{
		{
			name:  "bash has proper function definition",
			shell: ShellBash,
			checks: []string{
				"_hyperctl_completion()",
				"local ",
				"case ",
				"COMPREPLY",
			},
		},
		{
			name:  "zsh has proper function definition",
			shell: ShellZsh,
			checks: []string{
				"_hyperctl()",
				"local -a",
				"case ",
				"_arguments",
			},
		},
		{
			name:  "fish has proper completion syntax",
			shell: ShellFish,
			checks: []string{
				"complete -c hyperctl",
				"-n '__fish_use_subcommand'",
				"-n '__fish_seen_subcommand_from",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := Generate(tt.shell)
			if err != nil {
				t.Fatalf("failed to generate script: %v", err)
			}

			for _, check := range tt.checks {
				if !strings.Contains(script, check) {
					t.Errorf("script missing syntax element: %s", check)
				}
			}
		})
	}
}

func TestAllCommandsCovered(t *testing.T) {
	// All commands that should be in completion
	commands := []string{
		"submit",
		"query",
		"list",
		"grep",
		"rg",
		"vm",
		"status",
		"cancel",
		"migrate",
		"interactive",
		"help",
		"completion",
	}

	shells := []Shell{ShellBash, ShellZsh, ShellFish}

	for _, shell := range shells {
		t.Run(string(shell), func(t *testing.T) {
			script, err := Generate(shell)
			if err != nil {
				t.Fatalf("failed to generate script: %v", err)
			}

			for _, cmd := range commands {
				if !strings.Contains(script, cmd) {
					t.Errorf("completion for %s missing command: %s", shell, cmd)
				}
			}
		})
	}
}
