// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// DynamicK8sClient wraps Kubernetes dynamic client for CRD access
type DynamicK8sClient struct {
	client    dynamic.Interface
	namespace string
}

// NewDynamicK8sClient creates a new dynamic client
func NewDynamicK8sClient(config *rest.Config, namespace string) (*DynamicK8sClient, error) {
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &DynamicK8sClient{
		client:    client,
		namespace: namespace,
	}, nil
}

// BackupJob GVR (Group Version Resource)
var backupJobGVR = schema.GroupVersionResource{
	Group:    "transiva.zyvor.ai",
	Version:  "v1alpha1",
	Resource: "backupjobs",
}

// BackupSchedule GVR
var backupScheduleGVR = schema.GroupVersionResource{
	Group:    "transiva.zyvor.ai",
	Version:  "v1alpha1",
	Resource: "backupschedules",
}

// RestoreJob GVR
var restoreJobGVR = schema.GroupVersionResource{
	Group:    "transiva.zyvor.ai",
	Version:  "v1alpha1",
	Resource: "restorejobs",
}

// VirtualMachine GVR
var virtualMachineGVR = schema.GroupVersionResource{
	Group:    "transiva.zyvor.ai",
	Version:  "v1alpha1",
	Resource: "virtualmachines",
}

// VMTemplate GVR
var vmTemplateGVR = schema.GroupVersionResource{
	Group:    "transiva.zyvor.ai",
	Version:  "v1alpha1",
	Resource: "vmtemplates",
}

// VMSnapshot GVR
var vmSnapshotGVR = schema.GroupVersionResource{
	Group:    "transiva.zyvor.ai",
	Version:  "v1alpha1",
	Resource: "vmsnapshots",
}

// ListBackupJobs lists all BackupJobs
func (c *DynamicK8sClient) ListBackupJobs(ctx context.Context) (*unstructured.UnstructuredList, error) {
	if c.namespace == "" {
		return c.client.Resource(backupJobGVR).List(ctx, metav1.ListOptions{})
	}
	return c.client.Resource(backupJobGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
}

// GetBackupJob gets a specific BackupJob
func (c *DynamicK8sClient) GetBackupJob(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.client.Resource(backupJobGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// WatchBackupJobs watches for BackupJob changes
func (c *DynamicK8sClient) WatchBackupJobs(ctx context.Context) (watch.Interface, error) {
	if c.namespace == "" {
		return c.client.Resource(backupJobGVR).Watch(ctx, metav1.ListOptions{})
	}
	return c.client.Resource(backupJobGVR).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
}

// ListBackupSchedules lists all BackupSchedules
func (c *DynamicK8sClient) ListBackupSchedules(ctx context.Context) (*unstructured.UnstructuredList, error) {
	if c.namespace == "" {
		return c.client.Resource(backupScheduleGVR).List(ctx, metav1.ListOptions{})
	}
	return c.client.Resource(backupScheduleGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
}

// GetBackupSchedule gets a specific BackupSchedule
func (c *DynamicK8sClient) GetBackupSchedule(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.client.Resource(backupScheduleGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// WatchBackupSchedules watches for BackupSchedule changes
func (c *DynamicK8sClient) WatchBackupSchedules(ctx context.Context) (watch.Interface, error) {
	if c.namespace == "" {
		return c.client.Resource(backupScheduleGVR).Watch(ctx, metav1.ListOptions{})
	}
	return c.client.Resource(backupScheduleGVR).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
}

// ListRestoreJobs lists all RestoreJobs
func (c *DynamicK8sClient) ListRestoreJobs(ctx context.Context) (*unstructured.UnstructuredList, error) {
	if c.namespace == "" {
		return c.client.Resource(restoreJobGVR).List(ctx, metav1.ListOptions{})
	}
	return c.client.Resource(restoreJobGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
}

// GetRestoreJob gets a specific RestoreJob
func (c *DynamicK8sClient) GetRestoreJob(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.client.Resource(restoreJobGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// WatchRestoreJobs watches for RestoreJob changes
func (c *DynamicK8sClient) WatchRestoreJobs(ctx context.Context) (watch.Interface, error) {
	if c.namespace == "" {
		return c.client.Resource(restoreJobGVR).Watch(ctx, metav1.ListOptions{})
	}
	return c.client.Resource(restoreJobGVR).Namespace(c.namespace).Watch(ctx, metav1.ListOptions{})
}

// GetVirtualMachines lists all VirtualMachines
func (c *DynamicK8sClient) GetVirtualMachines(ctx context.Context, namespace string) ([]map[string]interface{}, error) {
	var list *unstructured.UnstructuredList
	var err error

	if namespace == "" || namespace == "all" {
		list, err = c.client.Resource(virtualMachineGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = c.client.Resource(virtualMachineGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, item.Object)
	}
	return result, nil
}

// GetVirtualMachine gets a specific VirtualMachine
func (c *DynamicK8sClient) GetVirtualMachine(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.client.Resource(virtualMachineGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetVMTemplates lists all VMTemplates
func (c *DynamicK8sClient) GetVMTemplates(ctx context.Context, namespace string) ([]map[string]interface{}, error) {
	var list *unstructured.UnstructuredList
	var err error

	if namespace == "" || namespace == "all" {
		list, err = c.client.Resource(vmTemplateGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = c.client.Resource(vmTemplateGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, item.Object)
	}
	return result, nil
}

// GetVMTemplate gets a specific VMTemplate
func (c *DynamicK8sClient) GetVMTemplate(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.client.Resource(vmTemplateGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetVMSnapshots lists all VMSnapshots
func (c *DynamicK8sClient) GetVMSnapshots(ctx context.Context, namespace string) ([]map[string]interface{}, error) {
	var list *unstructured.UnstructuredList
	var err error

	if namespace == "" || namespace == "all" {
		list, err = c.client.Resource(vmSnapshotGVR).List(ctx, metav1.ListOptions{})
	} else {
		list, err = c.client.Resource(vmSnapshotGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, item.Object)
	}
	return result, nil
}

// GetVMSnapshot gets a specific VMSnapshot
func (c *DynamicK8sClient) GetVMSnapshot(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.client.Resource(vmSnapshotGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// Helper functions to extract data from unstructured objects

// GetArrayField safely gets an array field from nested map
func GetArrayField(obj map[string]interface{}, fields ...string) []interface{} {
	current := obj
	for i, field := range fields {
		if i == len(fields)-1 {
			if val, ok := current[field].([]interface{}); ok {
				return val
			}
			return nil
		}
		if next, ok := current[field].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	return nil
}

// GetStringField safely gets a string field from nested map
func GetStringField(obj map[string]interface{}, fields ...string) string {
	current := obj
	for i, field := range fields {
		if i == len(fields)-1 {
			if val, ok := current[field].(string); ok {
				return val
			}
			return ""
		}
		if next, ok := current[field].(map[string]interface{}); ok {
			current = next
		} else {
			return ""
		}
	}
	return ""
}

// GetInt64Field safely gets an int64 field from nested map
func GetInt64Field(obj map[string]interface{}, fields ...string) int64 {
	current := obj
	for i, field := range fields {
		if i == len(fields)-1 {
			switch val := current[field].(type) {
			case int64:
				return val
			case int:
				return int64(val)
			case float64:
				return int64(val)
			default:
				return 0
			}
		}
		if next, ok := current[field].(map[string]interface{}); ok {
			current = next
		} else {
			return 0
		}
	}
	return 0
}

// GetFloat64Field safely gets a float64 field from nested map
func GetFloat64Field(obj map[string]interface{}, fields ...string) float64 {
	current := obj
	for i, field := range fields {
		if i == len(fields)-1 {
			switch val := current[field].(type) {
			case float64:
				return val
			case int64:
				return float64(val)
			case int:
				return float64(val)
			default:
				return 0.0
			}
		}
		if next, ok := current[field].(map[string]interface{}); ok {
			current = next
		} else {
			return 0.0
		}
	}
	return 0.0
}

// GetBoolField safely gets a bool field from nested map
func GetBoolField(obj map[string]interface{}, fields ...string) bool {
	current := obj
	for i, field := range fields {
		if i == len(fields)-1 {
			if val, ok := current[field].(bool); ok {
				return val
			}
			return false
		}
		if next, ok := current[field].(map[string]interface{}); ok {
			current = next
		} else {
			return false
		}
	}
	return false
}

// GetMapField safely gets a map field from nested map
func GetMapField(obj map[string]interface{}, fields ...string) map[string]interface{} {
	current := obj
	for i, field := range fields {
		if i == len(fields)-1 {
			if val, ok := current[field].(map[string]interface{}); ok {
				return val
			}
			return nil
		}
		if next, ok := current[field].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	return nil
}
