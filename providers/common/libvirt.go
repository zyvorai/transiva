// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/zyvorai/transiva/manifest"
)

// LibvirtConfig contains libvirt integration configuration
type LibvirtConfig struct {
	// URI is the libvirt connection URI
	// Examples: "qemu:///system", "qemu:///session", "qemu+ssh://host/system"
	URI string

	// AutoStart enables VM auto-start
	AutoStart bool

	// NetworkBridge is the default network bridge
	// Default: "virbr0"
	NetworkBridge string

	// StoragePool is the libvirt storage pool for disks
	// Default: "default"
	StoragePool string
}

// LibvirtIntegrator integrates converted VMs with libvirt
type LibvirtIntegrator struct {
	config *LibvirtConfig
	logger Logger
}

// NewLibvirtIntegrator creates a new libvirt integrator
func NewLibvirtIntegrator(config *LibvirtConfig, logger Logger) *LibvirtIntegrator {
	// Set defaults
	if config.URI == "" {
		config.URI = "qemu:///system"
	}
	if config.NetworkBridge == "" {
		config.NetworkBridge = "virbr0"
	}
	if config.StoragePool == "" {
		config.StoragePool = "default"
	}

	return &LibvirtIntegrator{
		config: config,
		logger: logger,
	}
}

// DefineVM defines a VM in libvirt from the manifest and converted disk
func (l *LibvirtIntegrator) DefineVM(ctx context.Context, m *manifest.ArtifactManifest, diskPath string) (string, error) {
	// Generate domain name from VM name or disk filename
	domainName := l.generateDomainName(m, diskPath)

	l.logger.Info("defining VM in libvirt", "domain", domainName, "uri", l.config.URI)

	// Generate libvirt XML
	xmlContent, err := l.generateLibvirtXML(m, diskPath, domainName)
	if err != nil {
		return "", fmt.Errorf("generate libvirt XML: %w", err)
	}

	// Write XML to temporary file
	xmlFile, err := os.CreateTemp("", "libvirt-domain-*.xml")
	if err != nil {
		return "", fmt.Errorf("create temp XML file: %w", err)
	}
	defer func() { _ = os.Remove(xmlFile.Name()) }()

	if _, err := xmlFile.WriteString(xmlContent); err != nil {
		return "", fmt.Errorf("write XML file: %w", err)
	}
	if err := xmlFile.Close(); err != nil {
		return "", fmt.Errorf("close temp XML file: %w", err)
	}

	// Define VM using virsh. l.config.URI comes from local LibvirtConfig (CLI/config),
	// and xmlFile.Name() is a system-generated temp path, neither is remote input.
	// #nosec G204 -- l.config.URI/xmlFile.Name() are local operator-supplied config and a generated temp path, not remote input
	cmd := exec.CommandContext(ctx, "virsh", "-c", l.config.URI, "define", xmlFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("virsh define failed: %w: %s", err, string(output))
	}

	l.logger.Info("VM defined successfully", "domain", domainName, "output", string(output))

	// Set auto-start if enabled
	if l.config.AutoStart {
		// #nosec G204 -- l.config.URI is local operator-supplied config and domainName is sanitized by sanitizeDomainName
		cmd = exec.CommandContext(ctx, "virsh", "-c", l.config.URI, "autostart", domainName)
		output, err = cmd.CombinedOutput()
		if err != nil {
			l.logger.Warn("failed to set autostart", "domain", domainName, "error", err, "output", string(output))
			// Non-fatal
		} else {
			l.logger.Info("VM autostart enabled", "domain", domainName)
		}
	}

	return domainName, nil
}

// generateDomainName generates a libvirt domain name
func (l *LibvirtIntegrator) generateDomainName(m *manifest.ArtifactManifest, diskPath string) string {
	// Prefer VM name from source metadata
	if m.Source != nil && m.Source.VMName != "" {
		return sanitizeDomainName(m.Source.VMName)
	}

	// Fallback to disk filename without extension
	baseName := filepath.Base(diskPath)
	ext := filepath.Ext(baseName)
	if ext != "" {
		baseName = baseName[:len(baseName)-len(ext)]
	}

	return sanitizeDomainName(baseName)
}

// sanitizeDomainName sanitizes a string for use as a libvirt domain name
func sanitizeDomainName(name string) string {
	// Remove invalid characters
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ToLower(name)

	// Remove non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			result.WriteRune(ch)
		}
	}

	sanitized := result.String()

	// Ensure it doesn't start with a hyphen or number
	sanitized = strings.TrimLeft(sanitized, "-0123456789")

	// Default if empty
	if sanitized == "" {
		sanitized = "vm"
	}

	return sanitized
}

// generateLibvirtXML generates libvirt domain XML
func (l *LibvirtIntegrator) generateLibvirtXML(m *manifest.ArtifactManifest, diskPath, domainName string) (string, error) {
	// Prepare template data
	data := l.prepareTemplateData(m, diskPath, domainName)

	// Parse and execute template
	tmpl, err := template.New("libvirt").Parse(libvirtXMLTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return result.String(), nil
}

// prepareTemplateData prepares data for the XML template
func (l *LibvirtIntegrator) prepareTemplateData(m *manifest.ArtifactManifest, diskPath, domainName string) map[string]interface{} {
	data := map[string]interface{}{
		"Name":          domainName,
		"Memory":        4 * 1024 * 1024, // 4 GB default
		"VCPU":          2,               // 2 vCPUs default
		"DiskPath":      diskPath,
		"DiskFormat":    "qcow2",
		"Firmware":      "bios",
		"NetworkBridge": l.config.NetworkBridge,
		"MACAddress":    "",
	}

	// Override with manifest data if available
	if m.VM != nil {
		if m.VM.CPU > 0 {
			data["VCPU"] = m.VM.CPU
		}
		if m.VM.MemGB > 0 {
			data["Memory"] = m.VM.MemGB * 1024 * 1024 // Convert GB to KiB
		}
		if m.VM.Firmware != "" {
			data["Firmware"] = m.VM.Firmware
		}
	}

	// Use first NIC MAC address if available
	if len(m.NICs) > 0 && m.NICs[0].MAC != "" {
		data["MACAddress"] = m.NICs[0].MAC
	}

	// Detect disk format from path
	ext := strings.ToLower(filepath.Ext(diskPath))
	switch ext {
	case ".qcow2":
		data["DiskFormat"] = "qcow2"
	case ".raw":
		data["DiskFormat"] = "raw"
	case ".img":
		data["DiskFormat"] = "raw"
	}

	return data
}

// libvirtXMLTemplate is the template for generating libvirt domain XML
const libvirtXMLTemplate = `<domain type='kvm'>
  <name>{{.Name}}</name>
  <memory unit='KiB'>{{.Memory}}</memory>
  <currentMemory unit='KiB'>{{.Memory}}</currentMemory>
  <vcpu placement='static'>{{.VCPU}}</vcpu>
  <os>{{if eq .Firmware "uefi"}}
    <type arch='x86_64' machine='q35'>hvm</type>
    <loader readonly='yes' type='pflash'>/usr/share/OVMF/OVMF_CODE.fd</loader>{{else}}
    <type arch='x86_64' machine='pc'>hvm</type>{{end}}
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>{{if eq .Firmware "uefi"}}
    <smm state='on'/>{{end}}
  </features>
  <cpu mode='host-passthrough' check='none' migratable='on'/>
  <clock offset='utc'>
    <timer name='rtc' tickpolicy='catchup'/>
    <timer name='pit' tickpolicy='delay'/>
    <timer name='hpet' present='no'/>
  </clock>
  <on_poweroff>destroy</on_poweroff>
  <on_reboot>restart</on_reboot>
  <on_crash>destroy</on_crash>
  <pm>
    <suspend-to-mem enabled='no'/>
    <suspend-to-disk enabled='no'/>
  </pm>
  <devices>
    <emulator>/usr/bin/qemu-system-x86_64</emulator>
    <disk type='file' device='disk'>
      <driver name='qemu' type='{{.DiskFormat}}' cache='writeback' discard='unmap'/>
      <source file='{{.DiskPath}}'/>
      <target dev='vda' bus='virtio'/>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x04' function='0x0'/>
    </disk>
    <controller type='usb' index='0' model='qemu-xhci' ports='15'>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x0'/>
    </controller>
    <controller type='pci' index='0' model='pcie-root'/>
    <controller type='virtio-serial' index='0'>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x03' function='0x0'/>
    </controller>
    <interface type='bridge'>
      <source bridge='{{.NetworkBridge}}'/>{{if .MACAddress}}
      <mac address='{{.MACAddress}}'/>{{end}}
      <model type='virtio'/>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x05' function='0x0'/>
    </interface>
    <serial type='pty'>
      <target type='isa-serial' port='0'>
        <model name='isa-serial'/>
      </target>
    </serial>
    <console type='pty'>
      <target type='serial' port='0'/>
    </console>
    <channel type='unix'>
      <target type='virtio' name='org.qemu.guest_agent.0'/>
      <address type='virtio-serial' controller='0' bus='0' port='1'/>
    </channel>
    <input type='tablet' bus='usb'>
      <address type='usb' bus='0' port='1'/>
    </input>
    <input type='mouse' bus='ps2'/>
    <input type='keyboard' bus='ps2'/>
    <graphics type='vnc' port='-1' autoport='yes' listen='127.0.0.1'>
      <listen type='address' address='127.0.0.1'/>
    </graphics>
    <video>
      <model type='qxl' ram='65536' vram='65536' vgamem='16384' heads='1' primary='yes'/>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x01' function='0x0'/>
    </video>
    <memballoon model='virtio'>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x06' function='0x0'/>
    </memballoon>
    <rng model='virtio'>
      <backend model='random'>/dev/urandom</backend>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x07' function='0x0'/>
    </rng>
  </devices>
</domain>
`
