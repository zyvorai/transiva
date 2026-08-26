// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

type VSphereClient struct {
	client  *govmomi.Client
	finder  *find.Finder
	config  *config.Config
	logger  logger.Logger
	retryer *retry.Retryer
}

func NewVSphereClient(ctx context.Context, cfg *config.Config, log logger.Logger) (*VSphereClient, error) {
	if cfg.VCenterURL == "" {
		return nil, fmt.Errorf("vCenter URL not configured (set GOVC_URL)")
	}

	// Parse vCenter URL
	u, err := soap.ParseURL(cfg.VCenterURL)
	if err != nil {
		return nil, fmt.Errorf("parse vCenter URL: %w", err)
	}

	// Set credentials
	u.User = url.UserPassword(cfg.Username, cfg.Password)

	// Create client with timeout
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	// Setup TLS config
	// #nosec G402 -- cfg.Insecure defaults to false (secure) and is only true when the operator explicitly opts in via GOVC_INSECURE=1, e.g. for self-signed vCenter certs in trusted lab environments
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.Insecure,
	}

	// Create SOAP client
	soapClient := soap.NewClient(u, cfg.Insecure)
	soapClient.DefaultTransport().TLSClientConfig = tlsConfig

	// Create vSphere client
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, fmt.Errorf("create vim25 client: %w", err)
	}

	// Create govmomi client
	client := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	// Create temporary retryer for login (before full client is created)
	loginRetryConfig := &retry.RetryConfig{
		MaxAttempts:  cfg.RetryAttempts,
		InitialDelay: cfg.RetryDelay,
		MaxDelay:     cfg.RetryDelay * 8,
		Multiplier:   2.0,
		Jitter:       true,
	}
	loginRetryer := retry.NewRetryer(loginRetryConfig, log)

	// Login with retry
	err = loginRetryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt > 1 {
			log.Info("retrying vCenter login", "attempt", attempt, "url", cfg.VCenterURL)
		}
		return client.Login(ctx, u.User)
	}, "vCenter login")

	if err != nil {
		return nil, fmt.Errorf("login to vCenter: %w", err)
	}

	// Create finder
	finder := find.NewFinder(client.Client, true)

	// Find default datacenter
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		log.Warn("no default datacenter found, using first available", "error", err)
		// Try to find any datacenter
		dcs, dcErr := finder.DatacenterList(ctx, "*")
		if dcErr != nil || len(dcs) == 0 {
			return nil, fmt.Errorf("find datacenter: %w", err)
		}
		dc = dcs[0]
	}

	finder.SetDatacenter(dc)

	log.Info("connected to vSphere",
		"url", cfg.VCenterURL,
		"datacenter", dc.Name())

	// Initialize retryer with vSphere-specific configuration
	retryConfig := &retry.RetryConfig{
		MaxAttempts:  cfg.RetryAttempts,
		InitialDelay: cfg.RetryDelay,
		MaxDelay:     cfg.RetryDelay * 16, // Cap at 16x initial delay
		Multiplier:   2.0,
		Jitter:       true,
	}
	retryer := retry.NewRetryer(retryConfig, log)

	return &VSphereClient{
		client:  client,
		finder:  finder,
		config:  cfg,
		logger:  log,
		retryer: retryer,
	}, nil
}

func (c *VSphereClient) Close() error {
	if c.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return c.client.Logout(ctx)
	}
	return nil
}
