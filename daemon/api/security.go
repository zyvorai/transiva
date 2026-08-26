// SPDX-License-Identifier: Apache-2.0

package api

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateWebhookURL validates a webhook URL and checks for SSRF vulnerabilities
func ValidateWebhookURL(webhookURL string, blockPrivateIPs bool) error {
	// Parse URL
	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: %s (must be http or https)", parsedURL.Scheme)
	}

	// Get hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("missing hostname in URL")
	}

	// If blocking private IPs, validate hostname
	if blockPrivateIPs {
		// Resolve hostname to IP addresses
		ips, err := net.LookupIP(hostname)
		if err != nil {
			return fmt.Errorf("failed to resolve hostname: %w", err)
		}

		// Check each resolved IP
		for _, ip := range ips {
			if isPrivateIP(ip) {
				return fmt.Errorf("webhook URL resolves to private IP address: %s", ip.String())
			}
		}
	}

	return nil
}

// isPrivateIP checks if an IP address is in a private range
func isPrivateIP(ip net.IP) bool {
	// Private IPv4 and IPv6 ranges (RFC 1918, RFC 4193, RFC 6598, RFC 5737, RFC 2544, etc.)
	privateRanges := []string{
		// IPv4 Private Networks
		"10.0.0.0/8",     // Class A private network (RFC 1918)
		"172.16.0.0/12",  // Class B private networks (RFC 1918)
		"192.168.0.0/16", // Class C private networks (RFC 1918)
		"127.0.0.0/8",    // Loopback (RFC 1122)
		"169.254.0.0/16", // Link-local (RFC 3927)
		"224.0.0.0/4",    // Multicast (RFC 5771)
		"240.0.0.0/4",    // Reserved (RFC 1112)
		"0.0.0.0/8",      // This network (RFC 1122)
		// Additional IPv4 Special-Purpose Ranges
		"100.64.0.0/10",   // Carrier-grade NAT (RFC 6598)
		"192.0.0.0/24",    // IETF Protocol Assignments (RFC 6890)
		"192.0.2.0/24",    // TEST-NET-1 (RFC 5737)
		"198.51.100.0/24", // TEST-NET-2 (RFC 5737)
		"203.0.113.0/24",  // TEST-NET-3 (RFC 5737)
		"198.18.0.0/15",   // Benchmark testing (RFC 2544)
		// IPv6 Private Networks
		"::1/128",       // IPv6 loopback (RFC 4291)
		"fe80::/10",     // IPv6 link-local (RFC 4291)
		"fc00::/7",      // IPv6 unique local addresses (RFC 4193)
		"::ffff:0:0/96", // IPv4-mapped IPv6 addresses (RFC 4291)
	}

	for _, cidr := range privateRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// SanitizeErrorMessage removes sensitive information from error messages
func SanitizeErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	// Remove file paths
	if strings.Contains(msg, "/") {
		parts := strings.Split(msg, ":")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Generic database errors
	if strings.Contains(strings.ToLower(msg), "sql") ||
		strings.Contains(strings.ToLower(msg), "database") {
		return "database error occurred"
	}

	// Generic file system errors
	if strings.Contains(strings.ToLower(msg), "permission denied") {
		return "permission error"
	}

	return "an error occurred"
}
