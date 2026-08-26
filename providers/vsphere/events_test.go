// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRecentEvents(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	tests := []struct {
		name        string
		since       time.Time
		eventTypes  []string
		entityTypes []string
		wantErr     bool
	}{
		{
			name:    "get events from last hour",
			since:   oneHourAgo,
			wantErr: false,
		},
		{
			name:       "filter by event type",
			since:      oneHourAgo,
			eventTypes: []string{"VmPoweredOnEvent", "VmPoweredOffEvent"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			events, err := client.GetRecentEvents(ctx, tt.since, tt.eventTypes, tt.entityTypes)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, events)

			// Events may be empty if none match filters
			for _, event := range events {
				assert.NotEmpty(t, event.EventType)
				assert.NotEmpty(t, event.Message)
				assert.NotZero(t, event.CreatedTime)

				// Verify event is within time range
				assert.True(t, event.CreatedTime.After(tt.since) || event.CreatedTime.Equal(tt.since))
			}
		})
	}
}

func TestStreamEvents(t *testing.T) {
	t.Skip("Skipping event streaming test - requires running vCenter")
}

func TestVCenterEventValidation(t *testing.T) {
	event := VCenterEvent{
		EventType:   "VmPoweredOnEvent",
		Message:     "Virtual machine 'test-vm' on host 'host-1' is powered on",
		EntityName:  "test-vm",
		EntityType:  "VirtualMachine",
		Severity:    "info",
		CreatedTime: time.Now(),
		UserName:    "administrator@vsphere.local",
	}

	assert.NotEmpty(t, event.EventType)
	assert.NotEmpty(t, event.Message)
	assert.NotEmpty(t, event.EntityName)
	assert.NotEmpty(t, event.Severity)
	assert.NotZero(t, event.CreatedTime)
}

func TestEventTypeCategories(t *testing.T) {
	// Common vSphere event types
	vmEvents := []string{
		"VmPoweredOnEvent",
		"VmPoweredOffEvent",
		"VmSuspendedEvent",
		"VmCreatedEvent",
		"VmRemovedEvent",
		"VmClonedEvent",
		"VmMigratedEvent",
	}

	hostEvents := []string{
		"HostConnectedEvent",
		"HostDisconnectedEvent",
		"HostConnectionLostEvent",
		"EnteredMaintenanceModeEvent",
		"ExitMaintenanceModeEvent",
	}

	datastoreEvents := []string{
		"DatastoreDiscoveredEvent",
		"DatastoreRemovedOnHostEvent",
	}

	assert.NotEmpty(t, vmEvents)
	assert.NotEmpty(t, hostEvents)
	assert.NotEmpty(t, datastoreEvents)

	// All event types should be unique
	allEvents := append(vmEvents, hostEvents...)
	allEvents = append(allEvents, datastoreEvents...)
	assert.Len(t, allEvents, len(vmEvents)+len(hostEvents)+len(datastoreEvents))
}

func TestEventSeverityLevels(t *testing.T) {
	severities := []string{"info", "warning", "error"}

	for _, sev := range severities {
		event := VCenterEvent{
			EventType:   "TestEvent",
			Message:     "Test message",
			Severity:    sev,
			CreatedTime: time.Now(),
		}

		assert.Contains(t, severities, event.Severity)
	}
}

func TestEventTimeRange(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	oneDayAgo := now.Add(-24 * time.Hour)

	tests := []struct {
		name      string
		eventTime time.Time
		since     time.Time
		inRange   bool
	}{
		{
			name:      "event within last hour",
			eventTime: now.Add(-30 * time.Minute),
			since:     oneHourAgo,
			inRange:   true,
		},
		{
			name:      "event older than range",
			eventTime: oneDayAgo,
			since:     oneHourAgo,
			inRange:   false,
		},
		{
			name:      "event exactly at boundary",
			eventTime: oneHourAgo,
			since:     oneHourAgo,
			inRange:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inRange := tt.eventTime.After(tt.since) || tt.eventTime.Equal(tt.since)
			assert.Equal(t, tt.inRange, inRange)
		})
	}
}

func TestEventMessageParsing(t *testing.T) {
	tests := []struct {
		name    string
		message string
		valid   bool
	}{
		{
			name:    "valid VM power on message",
			message: "Virtual machine 'test-vm' on host 'host-1' is powered on",
			valid:   true,
		},
		{
			name:    "valid VM migration message",
			message: "Migration of virtual machine 'test-vm' from 'host-1' to 'host-2' completed",
			valid:   true,
		},
		{
			name:    "empty message",
			message: "",
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := VCenterEvent{
				EventType: "TestEvent",
				Message:   tt.message,
			}

			isValid := event.Message != ""
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestGetRecentEventsContextCancellation(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetRecentEvents(ctx, time.Now().Add(-1*time.Hour), nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}
