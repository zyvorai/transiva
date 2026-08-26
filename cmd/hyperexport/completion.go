// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
)

// generateBashCompletion generates bash completion script
func generateBashCompletion() {
	script := `#!/bin/bash
# hyperexport bash completion script

_hyperexport_completions()
{
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # All flags
    opts="--vm --provider --output --format --compress --verify --dry-run --batch
          --filter --folder --power-off --parallel --quiet --version --interactive
          --tui --validate-only --resume --history --history-limit --report
          --report-file --clear-history --upload --stream-upload --keep-local
          --encrypt --encrypt-method --passphrase --keyfile --gpg-recipient
          --profile --save-profile --list-profiles --delete-profile
          --create-default-profiles --manifest --verify-manifest --manifest-checksum
          --manifest-target --convert --hyper2kvm-binary --conversion-timeout
          --stream-conversion --audit-log --daemon --daemon-port --daemon-addr
          --daemon-schedule --daemon-url --daemon-list --daemon-status --daemon-watch
          --snapshot --delete-snapshot --snapshot-name --snapshot-memory
          --snapshot-quiesce --keep-snapshots --consolidate-snapshots
          --bandwidth-limit --bandwidth-burst --adaptive-bandwidth
          --incremental --force-full --incremental-info
          --email-notify --email-smtp-host --email-smtp-port --email-from
          --email-to --email-username --email-password --email-on-start
          --email-on-complete --email-on-failure
          --cleanup --cleanup-max-age --cleanup-max-count --cleanup-max-size
          --cleanup-dry-run --cleanup-schedule
          --completion --help"

    # Context-aware completions
    case "${prev}" in
        --provider)
            COMPREPLY=( $(compgen -W "vsphere nutanix aws azure gcp hyperv" -- ${cur}) )
            return 0
            ;;
        --format)
            COMPREPLY=( $(compgen -W "ovf ova" -- ${cur}) )
            return 0
            ;;
        --encrypt-method)
            COMPREPLY=( $(compgen -W "aes256 gpg" -- ${cur}) )
            return 0
            ;;
        --manifest-target)
            COMPREPLY=( $(compgen -W "qcow2 raw vdi" -- ${cur}) )
            return 0
            ;;
        --daemon-list)
            COMPREPLY=( $(compgen -W "all running completed failed" -- ${cur}) )
            return 0
            ;;
        --completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- ${cur}) )
            return 0
            ;;
        --output|--batch|--report-file|--keyfile|--hyper2kvm-binary|--audit-log)
            # File/directory completion
            COMPREPLY=( $(compgen -f -- ${cur}) )
            return 0
            ;;
        --upload)
            # Cloud path completion hints
            if [[ ${cur} == s3://* ]]; then
                COMPREPLY=( "s3://bucket/path" )
            elif [[ ${cur} == azure://* ]]; then
                COMPREPLY=( "azure://container/path" )
            elif [[ ${cur} == gs://* ]]; then
                COMPREPLY=( "gs://bucket/path" )
            elif [[ ${cur} == sftp://* ]]; then
                COMPREPLY=( "sftp://host/path" )
            else
                COMPREPLY=( $(compgen -W "s3:// azure:// gs:// sftp://" -- ${cur}) )
            fi
            return 0
            ;;
    esac

    # Default flag completion
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
}

complete -F _hyperexport_completions hyperexport
`
	fmt.Print(script)
}

// generateZshCompletion generates zsh completion script
func generateZshCompletion() {
	script := `#compdef hyperexport
# hyperexport zsh completion script

_hyperexport() {
    local -a options
    options=(
        '--vm[VM name to export]:vm name:'
        '--provider[Provider type]:provider:(vsphere nutanix aws azure gcp hyperv)'
        '--output[Output directory]:directory:_files -/'
        '--format[Export format]:format:(ovf ova)'
        '--compress[Enable compression]'
        '--verify[Verify export with checksum]'
        '--dry-run[Preview export without exporting]'
        '--batch[Batch file with VM list]:file:_files'
        '--filter[Filter VMs by tag]:tag:'
        '--folder[Filter VMs by folder]:folder:'
        '--power-off[Power off VM before export]'
        '--parallel[Number of parallel downloads]:number:'
        '--quiet[Minimal output]'
        '--version[Show version]'
        '--interactive[Launch interactive TUI mode]'
        '--tui[Launch interactive TUI mode (alias)]'
        '--validate-only[Only run validation checks]'
        '--resume[Resume interrupted export]'
        '--history[Show export history]'
        '--history-limit[Number of exports to show]:number:'
        '--report[Generate statistics report]'
        '--report-file[Save report to file]:file:_files'
        '--clear-history[Clear export history]'
        '--upload[Upload to cloud]:path:'
        '--stream-upload[Stream directly to cloud]'
        '--keep-local[Keep local copy after upload]'
        '--encrypt[Encrypt export files]'
        '--encrypt-method[Encryption method]:method:(aes256 gpg)'
        '--passphrase[Encryption passphrase]:passphrase:'
        '--keyfile[Encryption key file]:file:_files'
        '--gpg-recipient[GPG recipient email]:email:'
        '--profile[Use saved profile]:profile:'
        '--save-profile[Save settings as profile]:name:'
        '--list-profiles[List available profiles]'
        '--delete-profile[Delete profile]:profile:'
        '--create-default-profiles[Create default profiles]'
        '--manifest[Generate Artifact Manifest v1.0]'
        '--verify-manifest[Verify manifest]'
        '--manifest-checksum[Compute checksums]'
        '--manifest-target[Target disk format]:format:(qcow2 raw vdi)'
        '--convert[Auto-convert with hyper2kvm]'
        '--hyper2kvm-binary[Path to hyper2kvm]:file:_files'
        '--conversion-timeout[Conversion timeout]:duration:'
        '--stream-conversion[Stream conversion output]'
        '--audit-log[Enable audit logging]:file:_files'
        '--daemon[Run in daemon mode]'
        '--daemon-port[Daemon port]:port:'
        '--daemon-addr[Daemon bind address]:address:'
        '--daemon-schedule[Create scheduled export]:schedule:'
        '--daemon-url[Daemon URL]:url:'
        '--daemon-list[List jobs]:filter:(all running completed failed)'
        '--daemon-status[Show daemon status]'
        '--daemon-watch[Watch job progress]:job-id:'
        '--snapshot[Create snapshot before export]'
        '--delete-snapshot[Delete snapshot after export]'
        '--snapshot-name[Custom snapshot name]:name:'
        '--snapshot-memory[Include memory in snapshot]'
        '--snapshot-quiesce[Quiesce filesystem]'
        '--keep-snapshots[Keep N snapshots]:number:'
        '--consolidate-snapshots[Consolidate snapshots]'
        '--bandwidth-limit[Bandwidth limit in MB/s]:mbps:'
        '--bandwidth-burst[Burst allowance in MB]:mb:'
        '--adaptive-bandwidth[Enable adaptive limiting]'
        '--incremental[Enable incremental export]'
        '--force-full[Force full export]'
        '--incremental-info[Show incremental analysis]'
        '--email-notify[Send email notifications]'
        '--email-smtp-host[SMTP server]:host:'
        '--email-smtp-port[SMTP port]:port:'
        '--email-from[From email address]:email:'
        '--email-to[To email addresses]:emails:'
        '--email-username[SMTP username]:username:'
        '--email-password[SMTP password]:password:'
        '--email-on-start[Email on start]'
        '--email-on-complete[Email on complete]'
        '--email-on-failure[Email on failure]'
        '--cleanup[Cleanup old exports]'
        '--cleanup-max-age[Delete older than]:duration:'
        '--cleanup-max-count[Keep N exports]:number:'
        '--cleanup-max-size[Max total size]:bytes:'
        '--cleanup-dry-run[Preview cleanup]'
        '--cleanup-schedule[Cleanup schedule]:duration:'
        '--completion[Generate completion script]:shell:(bash zsh fish)'
        '--help[Show help]'
    )

    _arguments -s -S $options
}

_hyperexport "$@"
`
	fmt.Print(script)
}

// generateFishCompletion generates fish completion script
func generateFishCompletion() {
	script := `# hyperexport fish completion script

# Format options
complete -c hyperexport -l format -d "Export format" -xa "ovf ova"
complete -c hyperexport -l provider -d "Provider type" -xa "vsphere nutanix aws azure gcp hyperv"
complete -c hyperexport -l encrypt-method -d "Encryption method" -xa "aes256 gpg"
complete -c hyperexport -l manifest-target -d "Target disk format" -xa "qcow2 raw vdi"
complete -c hyperexport -l daemon-list -d "List jobs" -xa "all running completed failed"
complete -c hyperexport -l completion -d "Generate completion" -xa "bash zsh fish"

# File/directory options
complete -c hyperexport -l output -d "Output directory" -r -F
complete -c hyperexport -l batch -d "Batch file" -r -F
complete -c hyperexport -l report-file -d "Report file" -r -F
complete -c hyperexport -l keyfile -d "Key file" -r -F
complete -c hyperexport -l hyper2kvm-binary -d "hyper2kvm binary" -r -F
complete -c hyperexport -l audit-log -d "Audit log file" -r -F

# String options
complete -c hyperexport -l vm -d "VM name"
complete -c hyperexport -l filter -d "Filter by tag"
complete -c hyperexport -l folder -d "Filter by folder"
complete -c hyperexport -l upload -d "Upload to cloud"
complete -c hyperexport -l passphrase -d "Encryption passphrase"
complete -c hyperexport -l gpg-recipient -d "GPG recipient"
complete -c hyperexport -l profile -d "Use profile"
complete -c hyperexport -l save-profile -d "Save profile"
complete -c hyperexport -l delete-profile -d "Delete profile"
complete -c hyperexport -l daemon-schedule -d "Scheduled export"
complete -c hyperexport -l daemon-url -d "Daemon URL"
complete -c hyperexport -l daemon-watch -d "Watch job"
complete -c hyperexport -l snapshot-name -d "Snapshot name"
complete -c hyperexport -l email-smtp-host -d "SMTP host"
complete -c hyperexport -l email-from -d "From email"
complete -c hyperexport -l email-to -d "To emails"
complete -c hyperexport -l email-username -d "SMTP username"
complete -c hyperexport -l email-password -d "SMTP password"

# Numeric options
complete -c hyperexport -l parallel -d "Parallel downloads"
complete -c hyperexport -l history-limit -d "History limit"
complete -c hyperexport -l daemon-port -d "Daemon port"
complete -c hyperexport -l keep-snapshots -d "Keep N snapshots"
complete -c hyperexport -l bandwidth-limit -d "Bandwidth limit (MB/s)"
complete -c hyperexport -l bandwidth-burst -d "Burst allowance (MB)"
complete -c hyperexport -l email-smtp-port -d "SMTP port"
complete -c hyperexport -l cleanup-max-count -d "Keep N exports"
complete -c hyperexport -l cleanup-max-size -d "Max size (bytes)"

# Boolean flags
complete -c hyperexport -l compress -d "Enable compression"
complete -c hyperexport -l verify -d "Verify export"
complete -c hyperexport -l dry-run -d "Preview only"
complete -c hyperexport -l power-off -d "Power off VM"
complete -c hyperexport -l quiet -d "Minimal output"
complete -c hyperexport -l version -d "Show version"
complete -c hyperexport -l interactive -d "Interactive TUI"
complete -c hyperexport -l tui -d "Interactive TUI"
complete -c hyperexport -l validate-only -d "Validate only"
complete -c hyperexport -l resume -d "Resume export"
complete -c hyperexport -l history -d "Show history"
complete -c hyperexport -l report -d "Generate report"
complete -c hyperexport -l clear-history -d "Clear history"
complete -c hyperexport -l stream-upload -d "Stream to cloud"
complete -c hyperexport -l keep-local -d "Keep local copy"
complete -c hyperexport -l encrypt -d "Encrypt files"
complete -c hyperexport -l list-profiles -d "List profiles"
complete -c hyperexport -l create-default-profiles -d "Create defaults"
complete -c hyperexport -l manifest -d "Generate manifest"
complete -c hyperexport -l verify-manifest -d "Verify manifest"
complete -c hyperexport -l manifest-checksum -d "Compute checksums"
complete -c hyperexport -l convert -d "Auto-convert"
complete -c hyperexport -l stream-conversion -d "Stream conversion"
complete -c hyperexport -l daemon -d "Daemon mode"
complete -c hyperexport -l daemon-status -d "Daemon status"
complete -c hyperexport -l snapshot -d "Create snapshot"
complete -c hyperexport -l delete-snapshot -d "Delete snapshot"
complete -c hyperexport -l snapshot-memory -d "Include memory"
complete -c hyperexport -l snapshot-quiesce -d "Quiesce filesystem"
complete -c hyperexport -l consolidate-snapshots -d "Consolidate snapshots"
complete -c hyperexport -l adaptive-bandwidth -d "Adaptive bandwidth"
complete -c hyperexport -l incremental -d "Incremental export"
complete -c hyperexport -l force-full -d "Force full export"
complete -c hyperexport -l incremental-info -d "Show analysis"
complete -c hyperexport -l email-notify -d "Email notifications"
complete -c hyperexport -l email-on-start -d "Email on start"
complete -c hyperexport -l email-on-complete -d "Email on complete"
complete -c hyperexport -l email-on-failure -d "Email on failure"
complete -c hyperexport -l cleanup -d "Cleanup old exports"
complete -c hyperexport -l cleanup-dry-run -d "Preview cleanup"
complete -c hyperexport -l help -d "Show help"
`
	fmt.Print(script)
}
