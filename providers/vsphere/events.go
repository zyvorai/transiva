// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/task"
	"github.com/vmware/govmomi/vim25/types"
)

// GetRecentEvents retrieves recent vCenter events
func (c *VSphereClient) GetRecentEvents(ctx context.Context, since time.Time, eventTypes, entityTypes []string) ([]VCenterEvent, error) {
	// Get event manager
	manager := event.NewManager(c.client.Client)

	// Build event filter
	filter := types.EventFilterSpec{
		Time: &types.EventFilterSpecByTime{
			BeginTime: &since,
		},
	}

	// Add event type filter if specified
	if len(eventTypes) > 0 {
		filter.EventTypeId = eventTypes
	}

	// Add entity filter if specified
	//
	// NOTE: vSphere's EventFilterSpec accepts only a single root entity plus
	// a recursion option, not a list of entity types, so entityTypes cannot
	// be applied as a per-type filter here. Recursion is still enabled so
	// events for all entities under the (implicit) root are returned.
	if len(entityTypes) > 0 {
		filter.Entity = &types.EventFilterSpecByEntity{
			Recursion: types.EventFilterSpecRecursionOptionAll,
		}
	}

	// Query events
	events, err := manager.QueryEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}

	// Convert to VCenterEvent
	result := make([]VCenterEvent, 0, len(events))
	for _, evt := range events {
		vcEvent := c.convertEvent(evt)
		result = append(result, vcEvent)
	}

	c.logger.Info("retrieved events",
		"count", len(result),
		"since", since.Format(time.RFC3339))

	return result, nil
}

// StreamEvents streams vCenter events in real-time
func (c *VSphereClient) StreamEvents(ctx context.Context, eventTypes, entityTypes []string) (<-chan VCenterEvent, error) {
	ch := make(chan VCenterEvent, 50)

	go func() {
		defer close(ch)

		// Poll events every 5 seconds
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		lastEventTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				c.logger.Info("event stream cancelled")
				return
			case <-ticker.C:
				events, err := c.GetRecentEvents(ctx, lastEventTime, eventTypes, entityTypes)
				if err != nil {
					c.logger.Error("failed to get events", "error", err)
					continue
				}

				// Send new events to channel
				for _, evt := range events {
					if evt.CreatedTime.After(lastEventTime) {
						lastEventTime = evt.CreatedTime
					}

					select {
					case ch <- evt:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	c.logger.Info("started event stream")
	return ch, nil
}

// GetRecentTasks retrieves recent vCenter tasks
func (c *VSphereClient) GetRecentTasks(ctx context.Context, since time.Time) ([]TaskInfo, error) {
	// Get task manager
	taskManager := task.NewManager(c.client.Client)

	// Get task history collector
	collector, err := taskManager.CreateCollectorForTasks(ctx, types.TaskFilterSpec{
		Time: &types.TaskFilterSpecByTime{
			BeginTime: &since,
			TimeType:  types.TaskFilterSpecTimeOptionStartedTime,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create task collector: %w", err)
	}
	defer func() { _ = collector.Destroy(ctx) }()

	// Read task history
	tasks, err := collector.ReadNextTasks(ctx, 1000) // Max 1000 tasks
	if err != nil {
		return nil, fmt.Errorf("read tasks: %w", err)
	}

	// Convert to TaskInfo
	result := make([]TaskInfo, 0, len(tasks))
	for _, t := range tasks {
		taskInfo := c.convertTask(&t)
		result = append(result, taskInfo)
	}

	c.logger.Info("retrieved tasks",
		"count", len(result),
		"since", since.Format(time.RFC3339))

	return result, nil
}

// Helper function to convert govmomi event to VCenterEvent
func (c *VSphereClient) convertEvent(evt types.BaseEvent) VCenterEvent {
	event := evt.GetEvent()

	vcEvent := VCenterEvent{
		EventID:     event.Key,
		EventType:   fmt.Sprintf("%T", evt),
		Message:     event.FullFormattedMessage,
		CreatedTime: event.CreatedTime,
		UserName:    event.UserName,
		ChainID:     event.ChainId,
		Metadata:    make(map[string]interface{}),
	}

	// Extract event type name (remove package prefix)
	eventType := fmt.Sprintf("%T", evt)
	if len(eventType) > 0 {
		// Remove "types." prefix if present
		if len(eventType) > 6 && eventType[:6] == "types." {
			vcEvent.EventType = eventType[6:]
		}
	}

	// Determine severity based on event type
	vcEvent.Severity = c.determineSeverity(vcEvent.EventType)

	// Extract entity information if available
	if event.Vm != nil {
		vcEvent.EntityType = "VirtualMachine"
		vcEvent.EntityName = event.Vm.Name
	} else if event.Host != nil {
		vcEvent.EntityType = "HostSystem"
		vcEvent.EntityName = event.Host.Name
	} else if event.ComputeResource != nil {
		vcEvent.EntityType = "ComputeResource"
		vcEvent.EntityName = event.ComputeResource.Name
	} else if event.Ds != nil {
		vcEvent.EntityType = "Datastore"
		vcEvent.EntityName = event.Ds.Name
	} else if event.Datacenter != nil {
		vcEvent.EntityType = "Datacenter"
		vcEvent.EntityName = event.Datacenter.Name
	}

	return vcEvent
}

// Helper function to convert task info
func (c *VSphereClient) convertTask(t *types.TaskInfo) TaskInfo {
	taskInfo := TaskInfo{
		TaskID:      t.Key,
		Name:        t.Name,
		Description: t.DescriptionId,
		State:       string(t.State),
		Progress:    t.Progress,
	}

	// Entity information
	if t.EntityName != "" {
		taskInfo.EntityName = t.EntityName
	}

	// Timestamps
	if t.StartTime != nil {
		taskInfo.StartTime = *t.StartTime
	}
	if t.CompleteTime != nil {
		taskInfo.CompleteTime = *t.CompleteTime
	}

	// Error information
	if t.Error != nil {
		taskInfo.Error = t.Error.LocalizedMessage
	}

	return taskInfo
}

// Helper function to determine event severity
func (c *VSphereClient) determineSeverity(eventType string) string {
	// Error events
	errorKeywords := []string{"Error", "Failed", "Fault", "Alarm"}
	for _, keyword := range errorKeywords {
		if containsIgnoreCase(eventType, keyword) {
			return "error"
		}
	}

	// Warning events
	warningKeywords := []string{"Warning", "Degraded", "Reconfigured", "Removed"}
	for _, keyword := range warningKeywords {
		if containsIgnoreCase(eventType, keyword) {
			return "warning"
		}
	}

	// Default to info
	return "info"
}

// Helper function for case-insensitive string contains
func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return contains(sLower, substrLower)
}

func toLower(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstring(s, substr) >= 0
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
