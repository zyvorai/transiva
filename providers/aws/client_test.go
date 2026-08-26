// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNewClient(t *testing.T) {
	cfg := &Config{
		Region:          "us-east-1",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		Timeout:         30 * time.Second,
	}

	log := logger.New("info")
	client, err := NewClient(cfg, log)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client == nil {
		t.Fatal("expected client to be created")
	}

	if client.config.Region != "us-east-1" {
		t.Errorf("expected region us-east-1, got %s", client.config.Region)
	}

	if client.config.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", client.config.Timeout)
	}
}

func TestNewClientDefaultTimeout(t *testing.T) {
	cfg := &Config{
		Region:          "us-west-2",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
	}

	log := logger.New("info")
	client, err := NewClient(cfg, log)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client.config.Timeout != 1*time.Hour {
		t.Errorf("expected default timeout 1h, got %v", client.config.Timeout)
	}
}

func TestClientString(t *testing.T) {
	cfg := &Config{
		Region:          "eu-west-1",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
	}

	log := logger.New("info")
	client, _ := NewClient(cfg, log)

	expected := "AWS EC2 Client (region=eu-west-1)"
	if client.String() != expected {
		t.Errorf("expected %s, got %s", expected, client.String())
	}
}

func TestVMInfoCreation(t *testing.T) {
	vmInfo := &VMInfo{
		InstanceID:   "i-1234567890abcdef0",
		Name:         "test-instance",
		State:        "running",
		InstanceType: "t2.micro",
		Platform:     "linux",
		PrivateIP:    "10.0.1.100",
		Tags:         map[string]string{"Environment": "test"},
	}

	if vmInfo.InstanceID != "i-1234567890abcdef0" {
		t.Errorf("unexpected instance ID: %s", vmInfo.InstanceID)
	}

	if vmInfo.Name != "test-instance" {
		t.Errorf("unexpected name: %s", vmInfo.Name)
	}

	if vmInfo.State != "running" {
		t.Errorf("unexpected state: %s", vmInfo.State)
	}

	if vmInfo.Tags["Environment"] != "test" {
		t.Errorf("unexpected tag value: %s", vmInfo.Tags["Environment"])
	}
}
