// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
	"os"
)

// GuestConfig defines guest OS configuration to inject during conversion
type GuestConfig struct {
	// Network configuration
	Network *NetworkConfig `json:"network,omitempty"`

	// User accounts
	Users []*UserConfig `json:"users,omitempty"`

	// SSH keys
	SSHKeys []*SSHKeyConfig `json:"ssh_keys,omitempty"`

	// Hostname
	Hostname string `json:"hostname,omitempty"`

	// Timezone
	Timezone string `json:"timezone,omitempty"`

	// Locale
	Locale string `json:"locale,omitempty"`

	// Custom scripts to run on first boot
	FirstBootScripts []string `json:"first_boot_scripts,omitempty"`

	// Packages to install on first boot
	Packages []string `json:"packages,omitempty"`

	// Cloud-init data (for cloud-init enabled images)
	CloudInit string `json:"cloud_init,omitempty"`
}

// NetworkConfig defines network configuration
type NetworkConfig struct {
	// Interfaces
	Interfaces []*NetworkInterface `json:"interfaces"`

	// DNS servers
	DNSServers []string `json:"dns_servers,omitempty"`

	// DNS search domains
	SearchDomains []string `json:"search_domains,omitempty"`

	// Default gateway
	DefaultGateway string `json:"default_gateway,omitempty"`
}

// NetworkInterface defines a network interface configuration
type NetworkInterface struct {
	// Interface name (e.g., eth0, ens3)
	Name string `json:"name"`

	// Configuration method: dhcp, static
	Method string `json:"method"`

	// Static IP configuration (if method is static)
	IPAddress string `json:"ip_address,omitempty"`
	Netmask   string `json:"netmask,omitempty"`
	Gateway   string `json:"gateway,omitempty"`

	// MAC address (optional, preserve from source or set custom)
	MACAddress string `json:"mac_address,omitempty"`

	// MTU
	MTU int `json:"mtu,omitempty"`

	// VLAN ID
	VLANID int `json:"vlan_id,omitempty"`
}

// UserConfig defines a user account configuration
type UserConfig struct {
	// Username
	Username string `json:"username"`

	// Password (hashed or plaintext, will be hashed if needed)
	Password string `json:"password,omitempty"`

	// PasswordHash (pre-hashed password)
	PasswordHash string `json:"password_hash,omitempty"`

	// Groups (additional groups for the user)
	Groups []string `json:"groups,omitempty"`

	// Sudo access
	Sudo bool `json:"sudo,omitempty"`

	// Shell (default: /bin/bash)
	Shell string `json:"shell,omitempty"`

	// Home directory (default: /home/username)
	Home string `json:"home,omitempty"`

	// SSH authorized keys
	SSHAuthorizedKeys []string `json:"ssh_authorized_keys,omitempty"`
}

// SSHKeyConfig defines SSH key configuration
type SSHKeyConfig struct {
	// User to add the key for
	User string `json:"user"`

	// Public key content
	PublicKey string `json:"public_key"`

	// Key type (optional: rsa, ed25519, ecdsa)
	KeyType string `json:"key_type,omitempty"`

	// Comment (optional)
	Comment string `json:"comment,omitempty"`
}

// LoadGuestConfig loads guest configuration from a file
func LoadGuestConfig(path string) (*GuestConfig, error) {
	// #nosec G304 -- path is supplied by the local operator via CLI/config, not remote/network input
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read guest config: %w", err)
	}

	var config GuestConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse guest config: %w", err)
	}

	return &config, nil
}

// Save saves guest configuration to a file
func (gc *GuestConfig) Save(path string) error {
	data, err := json.MarshalIndent(gc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal guest config: %w", err)
	}

	// GuestConfig may contain plaintext user passwords, so keep this file readable only by the owner.
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write guest config: %w", err)
	}

	return nil
}

// Validate validates the guest configuration
func (gc *GuestConfig) Validate() error {
	// Validate network config
	if gc.Network != nil {
		for i, iface := range gc.Network.Interfaces {
			if iface.Name == "" {
				return fmt.Errorf("network interface %d: name is required", i)
			}

			if iface.Method != "dhcp" && iface.Method != "static" {
				return fmt.Errorf("network interface %s: method must be 'dhcp' or 'static'", iface.Name)
			}

			if iface.Method == "static" {
				if iface.IPAddress == "" {
					return fmt.Errorf("network interface %s: ip_address is required for static method", iface.Name)
				}
				if iface.Netmask == "" {
					return fmt.Errorf("network interface %s: netmask is required for static method", iface.Name)
				}
			}
		}
	}

	// Validate users
	for i, user := range gc.Users {
		if user.Username == "" {
			return fmt.Errorf("user %d: username is required", i)
		}

		if user.Password == "" && user.PasswordHash == "" && len(user.SSHAuthorizedKeys) == 0 {
			return fmt.Errorf("user %s: must provide password, password_hash, or ssh_authorized_keys", user.Username)
		}
	}

	// Validate SSH keys
	for i, key := range gc.SSHKeys {
		if key.User == "" {
			return fmt.Errorf("ssh key %d: user is required", i)
		}
		if key.PublicKey == "" {
			return fmt.Errorf("ssh key %d: public_key is required", i)
		}
	}

	return nil
}

// ToCloudInit converts guest config to cloud-init user-data
func (gc *GuestConfig) ToCloudInit() (string, error) {
	cloudConfig := make(map[string]interface{})

	// Hostname
	if gc.Hostname != "" {
		cloudConfig["hostname"] = gc.Hostname
	}

	// Timezone
	if gc.Timezone != "" {
		cloudConfig["timezone"] = gc.Timezone
	}

	// Locale
	if gc.Locale != "" {
		cloudConfig["locale"] = gc.Locale
	}

	// Users
	if len(gc.Users) > 0 {
		var users []map[string]interface{}
		for _, user := range gc.Users {
			u := map[string]interface{}{
				"name": user.Username,
			}

			if user.PasswordHash != "" {
				u["passwd"] = user.PasswordHash
			} else if user.Password != "" {
				u["plain_text_passwd"] = user.Password
				u["lock_passwd"] = false
			}

			if len(user.Groups) > 0 {
				u["groups"] = user.Groups
			}

			if user.Sudo {
				u["sudo"] = "ALL=(ALL) NOPASSWD:ALL"
			}

			if user.Shell != "" {
				u["shell"] = user.Shell
			}

			if len(user.SSHAuthorizedKeys) > 0 {
				u["ssh_authorized_keys"] = user.SSHAuthorizedKeys
			}

			users = append(users, u)
		}
		cloudConfig["users"] = users
	}

	// SSH keys
	if len(gc.SSHKeys) > 0 {
		var sshKeys []string
		for _, key := range gc.SSHKeys {
			sshKeys = append(sshKeys, key.PublicKey)
		}
		cloudConfig["ssh_authorized_keys"] = sshKeys
	}

	// Packages
	if len(gc.Packages) > 0 {
		cloudConfig["packages"] = gc.Packages
	}

	// First boot scripts
	if len(gc.FirstBootScripts) > 0 {
		cloudConfig["runcmd"] = gc.FirstBootScripts
	}

	// Network config (v2 format)
	if gc.Network != nil && len(gc.Network.Interfaces) > 0 {
		networkConfig := make(map[string]interface{})
		networkConfig["version"] = 2

		ethernets := make(map[string]interface{})
		for _, iface := range gc.Network.Interfaces {
			ifaceConfig := make(map[string]interface{})

			switch iface.Method {
			case "dhcp":
				ifaceConfig["dhcp4"] = true
			case "static":
				ifaceConfig["addresses"] = []string{fmt.Sprintf("%s/%s", iface.IPAddress, iface.Netmask)}
				if iface.Gateway != "" {
					ifaceConfig["gateway4"] = iface.Gateway
				}
			}

			if iface.MACAddress != "" {
				ifaceConfig["match"] = map[string]string{"macaddress": iface.MACAddress}
			}

			if iface.MTU > 0 {
				ifaceConfig["mtu"] = iface.MTU
			}

			ethernets[iface.Name] = ifaceConfig
		}

		networkConfig["ethernets"] = ethernets

		if len(gc.Network.DNSServers) > 0 {
			networkConfig["nameservers"] = map[string]interface{}{
				"addresses": gc.Network.DNSServers,
			}
			if len(gc.Network.SearchDomains) > 0 {
				networkConfig["nameservers"].(map[string]interface{})["search"] = gc.Network.SearchDomains
			}
		}

		cloudConfig["network"] = networkConfig
	}

	data, err := json.MarshalIndent(cloudConfig, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal cloud-init config: %w", err)
	}

	return "#cloud-config\n" + string(data), nil
}

// NewDefaultGuestConfig creates a default guest configuration
func NewDefaultGuestConfig() *GuestConfig {
	return &GuestConfig{
		Hostname: "migrated-vm",
		Timezone: "UTC",
		Locale:   "en_US.UTF-8",
		Network: &NetworkConfig{
			Interfaces: []*NetworkInterface{
				{
					Name:   "eth0",
					Method: "dhcp",
				},
			},
		},
	}
}
