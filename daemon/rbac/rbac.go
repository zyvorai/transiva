// SPDX-License-Identifier: Apache-2.0

package rbac

import (
	"errors"
	"fmt"
	"sync"
)

// Role represents a user role
type Role string

const (
	RoleAdmin    Role = "admin"    // Full access to all resources
	RoleOperator Role = "operator" // Can export VMs and manage jobs
	RoleViewer   Role = "viewer"   // Read-only access
)

// Action represents an action that can be performed
type Action string

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionExport Action = "export"
	ActionCancel Action = "cancel"
)

// Resource represents a resource type
type Resource string

const (
	ResourceVM       Resource = "vm"
	ResourceJob      Resource = "job"
	ResourceSchedule Resource = "schedule"
	ResourceWebhook  Resource = "webhook"
	ResourceUser     Resource = "user"
	ResourceSystem   Resource = "system"
)

// Permission represents a single permission
type Permission struct {
	Resource Resource
	Action   Action
}

// String returns string representation of permission
func (p Permission) String() string {
	return fmt.Sprintf("%s:%s", p.Resource, p.Action)
}

// RolePermissions defines what each role can do
var RolePermissions = map[Role][]Permission{
	RoleAdmin: {
		// Full access to everything
		{ResourceVM, ActionCreate},
		{ResourceVM, ActionRead},
		{ResourceVM, ActionUpdate},
		{ResourceVM, ActionDelete},
		{ResourceVM, ActionExport},
		{ResourceJob, ActionCreate},
		{ResourceJob, ActionRead},
		{ResourceJob, ActionUpdate},
		{ResourceJob, ActionDelete},
		{ResourceJob, ActionCancel},
		{ResourceSchedule, ActionCreate},
		{ResourceSchedule, ActionRead},
		{ResourceSchedule, ActionUpdate},
		{ResourceSchedule, ActionDelete},
		{ResourceWebhook, ActionCreate},
		{ResourceWebhook, ActionRead},
		{ResourceWebhook, ActionUpdate},
		{ResourceWebhook, ActionDelete},
		{ResourceUser, ActionCreate},
		{ResourceUser, ActionRead},
		{ResourceUser, ActionUpdate},
		{ResourceUser, ActionDelete},
		{ResourceSystem, ActionRead},
		{ResourceSystem, ActionUpdate},
	},
	RoleOperator: {
		// Can manage VMs and jobs
		{ResourceVM, ActionRead},
		{ResourceVM, ActionExport},
		{ResourceJob, ActionCreate},
		{ResourceJob, ActionRead},
		{ResourceJob, ActionCancel},
		{ResourceSchedule, ActionRead},
		{ResourceWebhook, ActionRead},
		{ResourceUser, ActionRead},
		{ResourceSystem, ActionRead},
	},
	RoleViewer: {
		// Read-only access
		{ResourceVM, ActionRead},
		{ResourceJob, ActionRead},
		{ResourceSchedule, ActionRead},
		{ResourceWebhook, ActionRead},
		{ResourceUser, ActionRead},
		{ResourceSystem, ActionRead},
	},
}

// User represents a user with roles
type User struct {
	Username string
	Roles    []Role
}

// RBACManager manages role-based access control
type RBACManager struct {
	users map[string]*User
	mu    sync.RWMutex
}

// NewRBACManager creates a new RBAC manager
func NewRBACManager() *RBACManager {
	return &RBACManager{
		users: make(map[string]*User),
	}
}

// AddUser adds a user with specified roles
func (m *RBACManager) AddUser(username string, roles ...Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if username == "" {
		return errors.New("username cannot be empty")
	}

	// Validate roles
	for _, role := range roles {
		if !isValidRole(role) {
			return fmt.Errorf("invalid role: %s", role)
		}
	}

	m.users[username] = &User{
		Username: username,
		Roles:    roles,
	}

	return nil
}

// RemoveUser removes a user
func (m *RBACManager) RemoveUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[username]; !exists {
		return fmt.Errorf("user not found: %s", username)
	}

	delete(m.users, username)
	return nil
}

// GetUser retrieves a user
func (m *RBACManager) GetUser(username string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	return user, nil
}

// UpdateUserRoles updates a user's roles
func (m *RBACManager) UpdateUserRoles(username string, roles ...Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return fmt.Errorf("user not found: %s", username)
	}

	// Validate roles
	for _, role := range roles {
		if !isValidRole(role) {
			return fmt.Errorf("invalid role: %s", role)
		}
	}

	user.Roles = roles
	return nil
}

// CheckPermission checks if a user has permission to perform an action on a resource
func (m *RBACManager) CheckPermission(username string, resource Resource, action Action) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return fmt.Errorf("user not found: %s", username)
	}

	// Check if user has any role with the required permission
	for _, role := range user.Roles {
		permissions, exists := RolePermissions[role]
		if !exists {
			continue
		}

		for _, perm := range permissions {
			if perm.Resource == resource && perm.Action == action {
				return nil // Permission granted
			}
		}
	}

	return fmt.Errorf("user %s does not have permission %s:%s", username, resource, action)
}

// HasRole checks if a user has a specific role
func (m *RBACManager) HasRole(username string, role Role) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return false
	}

	for _, r := range user.Roles {
		if r == role {
			return true
		}
	}

	return false
}

// ListUsers returns all users
func (m *RBACManager) ListUsers() []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]*User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}

	return users
}

// GetUserPermissions returns all permissions for a user
func (m *RBACManager) GetUserPermissions(username string) ([]Permission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	permMap := make(map[string]Permission)
	for _, role := range user.Roles {
		permissions, exists := RolePermissions[role]
		if !exists {
			continue
		}

		for _, perm := range permissions {
			permMap[perm.String()] = perm
		}
	}

	// Convert map to slice
	perms := make([]Permission, 0, len(permMap))
	for _, perm := range permMap {
		perms = append(perms, perm)
	}

	return perms, nil
}

// isValidRole checks if a role is valid
func isValidRole(role Role) bool {
	switch role {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	default:
		return false
	}
}

// GetAllRoles returns all available roles
func GetAllRoles() []Role {
	return []Role{RoleAdmin, RoleOperator, RoleViewer}
}

// GetRoleDescription returns a description of a role
func GetRoleDescription(role Role) string {
	switch role {
	case RoleAdmin:
		return "Full administrative access to all resources and operations"
	case RoleOperator:
		return "Can export VMs, manage jobs, and view system status"
	case RoleViewer:
		return "Read-only access to VMs, jobs, schedules, and system status"
	default:
		return "Unknown role"
	}
}

// ResourcePermissions represents permissions for a specific resource
type ResourcePermissions struct {
	Resource Resource
	Create   bool
	Read     bool
	Update   bool
	Delete   bool
	Export   bool
	Cancel   bool
}

// GetUserResourcePermissions returns permissions for a user on a specific resource
func (m *RBACManager) GetUserResourcePermissions(username string, resource Resource) (*ResourcePermissions, error) {
	permissions, err := m.GetUserPermissions(username)
	if err != nil {
		return nil, err
	}

	rp := &ResourcePermissions{
		Resource: resource,
	}

	for _, perm := range permissions {
		if perm.Resource == resource {
			switch perm.Action {
			case ActionCreate:
				rp.Create = true
			case ActionRead:
				rp.Read = true
			case ActionUpdate:
				rp.Update = true
			case ActionDelete:
				rp.Delete = true
			case ActionExport:
				rp.Export = true
			case ActionCancel:
				rp.Cancel = true
			}
		}
	}

	return rp, nil
}
