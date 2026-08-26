// SPDX-License-Identifier: Apache-2.0

package rbac

import (
	"testing"
)

func TestNewRBACManager(t *testing.T) {
	manager := NewRBACManager()
	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	users := manager.ListUsers()
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}

func TestAddUser(t *testing.T) {
	manager := NewRBACManager()

	// Add admin user
	err := manager.AddUser("alice", RoleAdmin)
	if err != nil {
		t.Fatalf("failed to add user: %v", err)
	}

	// Add operator user
	err = manager.AddUser("bob", RoleOperator)
	if err != nil {
		t.Fatalf("failed to add user: %v", err)
	}

	// Add viewer user
	err = manager.AddUser("charlie", RoleViewer)
	if err != nil {
		t.Fatalf("failed to add user: %v", err)
	}

	users := manager.ListUsers()
	if len(users) != 3 {
		t.Errorf("expected 3 users, got %d", len(users))
	}
}

func TestAddUserInvalidRole(t *testing.T) {
	manager := NewRBACManager()

	err := manager.AddUser("dave", Role("invalid"))
	if err == nil {
		t.Error("expected error for invalid role")
	}
}

func TestAddUserEmptyUsername(t *testing.T) {
	manager := NewRBACManager()

	err := manager.AddUser("", RoleAdmin)
	if err == nil {
		t.Error("expected error for empty username")
	}
}

func TestRemoveUser(t *testing.T) {
	manager := NewRBACManager()

	_ = manager.AddUser("alice", RoleAdmin)

	err := manager.RemoveUser("alice")
	if err != nil {
		t.Fatalf("failed to remove user: %v", err)
	}

	_, err = manager.GetUser("alice")
	if err == nil {
		t.Error("expected error when getting removed user")
	}
}

func TestGetUser(t *testing.T) {
	manager := NewRBACManager()

	_ = manager.AddUser("alice", RoleAdmin, RoleOperator)

	user, err := manager.GetUser("alice")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if user.Username != "alice" {
		t.Errorf("expected username alice, got %s", user.Username)
	}

	if len(user.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(user.Roles))
	}
}

func TestUpdateUserRoles(t *testing.T) {
	manager := NewRBACManager()

	_ = manager.AddUser("alice", RoleViewer)

	err := manager.UpdateUserRoles("alice", RoleAdmin)
	if err != nil {
		t.Fatalf("failed to update user roles: %v", err)
	}

	user, _ := manager.GetUser("alice")
	if len(user.Roles) != 1 || user.Roles[0] != RoleAdmin {
		t.Error("user roles not updated correctly")
	}
}

func TestCheckPermission(t *testing.T) {
	manager := NewRBACManager()

	// Add users with different roles
	_ = manager.AddUser("admin", RoleAdmin)
	_ = manager.AddUser("operator", RoleOperator)
	_ = manager.AddUser("viewer", RoleViewer)

	tests := []struct {
		username   string
		resource   Resource
		action     Action
		shouldPass bool
	}{
		// Admin tests
		{"admin", ResourceVM, ActionCreate, true},
		{"admin", ResourceVM, ActionRead, true},
		{"admin", ResourceVM, ActionDelete, true},
		{"admin", ResourceJob, ActionCreate, true},
		{"admin", ResourceUser, ActionDelete, true},

		// Operator tests
		{"operator", ResourceVM, ActionRead, true},
		{"operator", ResourceVM, ActionExport, true},
		{"operator", ResourceJob, ActionCreate, true},
		{"operator", ResourceJob, ActionCancel, true},
		{"operator", ResourceVM, ActionDelete, false},   // No delete permission
		{"operator", ResourceUser, ActionCreate, false}, // No user management

		// Viewer tests
		{"viewer", ResourceVM, ActionRead, true},
		{"viewer", ResourceJob, ActionRead, true},
		{"viewer", ResourceVM, ActionCreate, false},  // Read-only
		{"viewer", ResourceJob, ActionDelete, false}, // Read-only
		{"viewer", ResourceVM, ActionExport, false},  // Read-only
	}

	for _, tt := range tests {
		err := manager.CheckPermission(tt.username, tt.resource, tt.action)
		if tt.shouldPass && err != nil {
			t.Errorf("expected permission to pass for %s on %s:%s, got error: %v",
				tt.username, tt.resource, tt.action, err)
		}
		if !tt.shouldPass && err == nil {
			t.Errorf("expected permission to fail for %s on %s:%s",
				tt.username, tt.resource, tt.action)
		}
	}
}

func TestHasRole(t *testing.T) {
	manager := NewRBACManager()

	_ = manager.AddUser("alice", RoleAdmin, RoleOperator)

	if !manager.HasRole("alice", RoleAdmin) {
		t.Error("expected alice to have admin role")
	}

	if !manager.HasRole("alice", RoleOperator) {
		t.Error("expected alice to have operator role")
	}

	if manager.HasRole("alice", RoleViewer) {
		t.Error("expected alice not to have viewer role")
	}

	if manager.HasRole("bob", RoleAdmin) {
		t.Error("expected nonexistent user to not have any role")
	}
}

func TestGetUserPermissions(t *testing.T) {
	manager := NewRBACManager()

	_ = manager.AddUser("alice", RoleAdmin)

	perms, err := manager.GetUserPermissions("alice")
	if err != nil {
		t.Fatalf("failed to get permissions: %v", err)
	}

	if len(perms) == 0 {
		t.Error("expected admin to have permissions")
	}

	// Admin should have many permissions
	if len(perms) < 10 {
		t.Errorf("expected admin to have at least 10 permissions, got %d", len(perms))
	}
}

func TestGetUserResourcePermissions(t *testing.T) {
	manager := NewRBACManager()

	_ = manager.AddUser("operator", RoleOperator)

	rp, err := manager.GetUserResourcePermissions("operator", ResourceVM)
	if err != nil {
		t.Fatalf("failed to get resource permissions: %v", err)
	}

	if !rp.Read {
		t.Error("expected operator to have read permission on VMs")
	}

	if !rp.Export {
		t.Error("expected operator to have export permission on VMs")
	}

	if rp.Create {
		t.Error("expected operator not to have create permission on VMs")
	}

	if rp.Delete {
		t.Error("expected operator not to have delete permission on VMs")
	}
}

func TestGetAllRoles(t *testing.T) {
	roles := GetAllRoles()

	if len(roles) != 3 {
		t.Errorf("expected 3 roles, got %d", len(roles))
	}

	hasAdmin := false
	hasOperator := false
	hasViewer := false

	for _, role := range roles {
		switch role {
		case RoleAdmin:
			hasAdmin = true
		case RoleOperator:
			hasOperator = true
		case RoleViewer:
			hasViewer = true
		}
	}

	if !hasAdmin || !hasOperator || !hasViewer {
		t.Error("expected all three roles to be present")
	}
}

func TestGetRoleDescription(t *testing.T) {
	desc := GetRoleDescription(RoleAdmin)
	if desc == "" || desc == "Unknown role" {
		t.Error("expected valid description for admin role")
	}

	desc = GetRoleDescription(RoleOperator)
	if desc == "" || desc == "Unknown role" {
		t.Error("expected valid description for operator role")
	}

	desc = GetRoleDescription(RoleViewer)
	if desc == "" || desc == "Unknown role" {
		t.Error("expected valid description for viewer role")
	}

	desc = GetRoleDescription(Role("invalid"))
	if desc != "Unknown role" {
		t.Error("expected 'Unknown role' for invalid role")
	}
}

func TestPermissionString(t *testing.T) {
	perm := Permission{
		Resource: ResourceVM,
		Action:   ActionCreate,
	}

	expected := "vm:create"
	if perm.String() != expected {
		t.Errorf("expected %s, got %s", expected, perm.String())
	}
}

func TestGetUserResourcePermissions_AllActions(t *testing.T) {
	manager := NewRBACManager()

	// Admin should have all permissions on VMs
	_ = manager.AddUser("admin", RoleAdmin)

	rpVM, err := manager.GetUserResourcePermissions("admin", ResourceVM)
	if err != nil {
		t.Fatalf("failed to get VM resource permissions: %v", err)
	}

	// Check VM action types
	if !rpVM.Create {
		t.Error("expected admin to have Create permission on VMs")
	}

	if !rpVM.Read {
		t.Error("expected admin to have Read permission on VMs")
	}

	if !rpVM.Update {
		t.Error("expected admin to have Update permission on VMs")
	}

	if !rpVM.Delete {
		t.Error("expected admin to have Delete permission on VMs")
	}

	if !rpVM.Export {
		t.Error("expected admin to have Export permission on VMs")
	}

	// Check Jobs resource for Cancel action
	rpJob, err := manager.GetUserResourcePermissions("admin", ResourceJob)
	if err != nil {
		t.Fatalf("failed to get Job resource permissions: %v", err)
	}

	if !rpJob.Cancel {
		t.Error("expected admin to have Cancel permission on Jobs")
	}

	if !rpJob.Create {
		t.Error("expected admin to have Create permission on Jobs")
	}
}

func TestGetUserResourcePermissions_NonexistentUser(t *testing.T) {
	manager := NewRBACManager()

	_, err := manager.GetUserResourcePermissions("nonexistent", ResourceVM)
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}
