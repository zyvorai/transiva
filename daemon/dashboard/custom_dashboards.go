// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// dashboardIDRe restricts dashboard IDs to a safe character set so a
// crafted ID (e.g. containing "../") cannot be used to escape the storage
// directory when building a dashboard file path.
var dashboardIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

// validateDashboardID validates a dashboard ID before it is used to build
// a file path under the storage directory.
func validateDashboardID(id string) error {
	if !dashboardIDRe.MatchString(id) {
		return fmt.Errorf("invalid dashboard id %q", id)
	}
	return nil
}

// CustomDashboard represents a user-defined dashboard layout
type CustomDashboard struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Layout      DashboardLayout `json:"layout"`
	IsDefault   bool            `json:"isDefault"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	Owner       string          `json:"owner,omitempty"`
	Shared      bool            `json:"shared"`
}

// DashboardLayout defines the dashboard grid layout
type DashboardLayout struct {
	Columns int             `json:"columns"` // Grid columns (default: 12)
	Rows    int             `json:"rows"`    // Grid rows
	Widgets []WidgetConfig  `json:"widgets"`
}

// WidgetConfig defines a widget position and configuration
type WidgetConfig struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // chart, table, metric, console, etc.
	Title    string                 `json:"title"`
	X        int                    `json:"x"`      // Column position
	Y        int                    `json:"y"`      // Row position
	Width    int                    `json:"width"`  // Widget width in columns
	Height   int                    `json:"height"` // Widget height in rows
	Config   map[string]interface{} `json:"config,omitempty"` // Widget-specific config
	Refresh  int                    `json:"refresh,omitempty"` // Auto-refresh interval in seconds
}

// CustomDashboardManager manages custom dashboards
type CustomDashboardManager struct {
	dashboards map[string]*CustomDashboard
	mu         sync.RWMutex
	storageDir string
}

// NewCustomDashboardManager creates a new dashboard manager
func NewCustomDashboardManager(storageDir string) (*CustomDashboardManager, error) {
	// Ensure storage directory exists
	if err := os.MkdirAll(storageDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	manager := &CustomDashboardManager{
		dashboards: make(map[string]*CustomDashboard),
		storageDir: storageDir,
	}

	// Load existing dashboards from disk
	if err := manager.loadDashboards(); err != nil {
		return nil, err
	}

	// Create default dashboard if none exist
	if len(manager.dashboards) == 0 {
		if err := manager.createDefaultDashboard(); err != nil {
			return nil, fmt.Errorf("failed to create default dashboard: %w", err)
		}
	}

	return manager, nil
}

// loadDashboards loads dashboards from disk
func (m *CustomDashboardManager) loadDashboards() error {
	files, err := filepath.Glob(filepath.Join(m.storageDir, "dashboard-*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		// #nosec G304 -- file comes from filepath.Glob restricted to storageDir with a fixed "dashboard-*.json" pattern, not from external input
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var dashboard CustomDashboard
		if err := json.Unmarshal(data, &dashboard); err != nil {
			continue
		}

		m.dashboards[dashboard.ID] = &dashboard
	}

	return nil
}

// saveDashboard saves a dashboard to disk
func (m *CustomDashboardManager) saveDashboard(dashboard *CustomDashboard) error {
	if err := validateDashboardID(dashboard.ID); err != nil {
		return err
	}

	data, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		return err
	}

	filename := filepath.Join(m.storageDir, fmt.Sprintf("dashboard-%s.json", dashboard.ID))
	return os.WriteFile(filename, data, 0600)
}

// createDefaultDashboard creates a default dashboard layout
func (m *CustomDashboardManager) createDefaultDashboard() error {
	defaultDashboard := &CustomDashboard{
		ID:          "default",
		Name:        "Default Dashboard",
		Description: "Default HyperSDK dashboard with all metrics",
		IsDefault:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Shared:      true,
		Layout: DashboardLayout{
			Columns: 12,
			Rows:    6,
			Widgets: []WidgetConfig{
				{
					ID:     "vm-overview",
					Type:   "metric",
					Title:  "VM Overview",
					X:      0,
					Y:      0,
					Width:  3,
					Height: 1,
					Config: map[string]interface{}{
						"metric": "vm_total",
					},
					Refresh: 5,
				},
				{
					ID:     "vm-status-chart",
					Type:   "chart",
					Title:  "VM Status Distribution",
					X:      3,
					Y:      0,
					Width:  3,
					Height: 2,
					Config: map[string]interface{}{
						"chartType": "pie",
						"metric":    "vm_by_status",
					},
					Refresh: 10,
				},
				{
					ID:     "resource-usage",
					Type:   "chart",
					Title:  "Resource Usage",
					X:      6,
					Y:      0,
					Width:  6,
					Height: 2,
					Config: map[string]interface{}{
						"chartType": "line",
						"metrics":   []string{"cpu_usage", "memory_usage"},
					},
					Refresh: 5,
				},
				{
					ID:     "vm-list",
					Type:   "table",
					Title:  "Virtual Machines",
					X:      0,
					Y:      2,
					Width:  12,
					Height: 4,
					Config: map[string]interface{}{
						"columns": []string{"name", "status", "cpus", "memory", "node"},
					},
					Refresh: 10,
				},
			},
		},
	}

	m.mu.Lock()
	m.dashboards[defaultDashboard.ID] = defaultDashboard
	m.mu.Unlock()

	return m.saveDashboard(defaultDashboard)
}

// GetDashboard retrieves a dashboard by ID
func (m *CustomDashboardManager) GetDashboard(id string) (*CustomDashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard, ok := m.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard not found: %s", id)
	}

	return dashboard, nil
}

// ListDashboards lists all dashboards
func (m *CustomDashboardManager) ListDashboards() []*CustomDashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboards := make([]*CustomDashboard, 0, len(m.dashboards))
	for _, dashboard := range m.dashboards {
		dashboards = append(dashboards, dashboard)
	}

	return dashboards
}

// CreateDashboard creates a new dashboard
func (m *CustomDashboardManager) CreateDashboard(dashboard *CustomDashboard) error {
	if dashboard.ID == "" {
		dashboard.ID = fmt.Sprintf("dashboard-%d", time.Now().Unix())
	}

	dashboard.CreatedAt = time.Now()
	dashboard.UpdatedAt = time.Now()

	m.mu.Lock()
	m.dashboards[dashboard.ID] = dashboard
	m.mu.Unlock()

	return m.saveDashboard(dashboard)
}

// UpdateDashboard updates an existing dashboard
func (m *CustomDashboardManager) UpdateDashboard(id string, dashboard *CustomDashboard) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.dashboards[id]
	if !ok {
		return fmt.Errorf("dashboard not found: %s", id)
	}

	// Preserve creation time and ID
	dashboard.ID = existing.ID
	dashboard.CreatedAt = existing.CreatedAt
	dashboard.UpdatedAt = time.Now()

	m.dashboards[id] = dashboard

	return m.saveDashboard(dashboard)
}

// DeleteDashboard deletes a dashboard
func (m *CustomDashboardManager) DeleteDashboard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.dashboards[id]; !ok {
		return fmt.Errorf("dashboard not found: %s", id)
	}

	// Don't allow deleting the default dashboard
	if id == "default" {
		return fmt.Errorf("cannot delete default dashboard")
	}

	if err := validateDashboardID(id); err != nil {
		return err
	}

	delete(m.dashboards, id)

	// Delete from disk
	filename := filepath.Join(m.storageDir, fmt.Sprintf("dashboard-%s.json", id))
	// #nosec G703 -- id is validated by validateDashboardID above against an allowlist regex, preventing path traversal
	return os.Remove(filename)
}

// RegisterCustomDashboardChiRoutes registers custom dashboard API routes with chi router
func RegisterCustomDashboardChiRoutes(r chi.Router, manager *CustomDashboardManager) {
	r.Route("/dashboards", func(r chi.Router) {
		// List all dashboards
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			dashboards := manager.ListDashboards()
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(dashboards); err != nil {
				log.Printf("failed to encode dashboards response: %v", err)
			}
		})

		// Create new dashboard
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var dashboard CustomDashboard
			if err := json.NewDecoder(r.Body).Decode(&dashboard); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if err := manager.CreateDashboard(&dashboard); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(dashboard); err != nil {
				log.Printf("failed to encode created dashboard response: %v", err)
			}
		})

		// Dashboard-specific operations
		r.Route("/{id}", func(r chi.Router) {
			// Get specific dashboard
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				dashboard, err := manager.GetDashboard(id)
				if err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(dashboard); err != nil {
					log.Printf("failed to encode dashboard response: %v", err)
				}
			})

			// Update dashboard
			r.Put("/", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				var dashboard CustomDashboard
				if err := json.NewDecoder(r.Body).Decode(&dashboard); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				if err := manager.UpdateDashboard(id, &dashboard); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(dashboard); err != nil {
					log.Printf("failed to encode updated dashboard response: %v", err)
				}
			})

			// Delete dashboard
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				if err := manager.DeleteDashboard(id); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			// Clone dashboard
			r.Post("/clone", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				original, err := manager.GetDashboard(id)
				if err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}

				// Create a copy
				clone := *original
				clone.ID = fmt.Sprintf("%s-clone-%d", id, time.Now().Unix())
				clone.Name = fmt.Sprintf("%s (Copy)", original.Name)
				clone.IsDefault = false

				if err := manager.CreateDashboard(&clone); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				if err := json.NewEncoder(w).Encode(clone); err != nil {
					log.Printf("failed to encode cloned dashboard response: %v", err)
				}
			})
		})
	})
}
