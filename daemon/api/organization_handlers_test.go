// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// List Tags Handler Tests

func TestHandleListTagsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/tags", nil)
	w := httptest.NewRecorder()

	server.handleListTags(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListTags(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/tags", nil)
	w := httptest.NewRecorder()

	server.handleListTags(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["tags"]; !ok {
		t.Error("Expected tags field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 7 {
		t.Errorf("Expected total=7, got %v", total)
	}

	// Verify tag structure
	tags := response["tags"].([]interface{})
	if len(tags) != 7 {
		t.Errorf("Expected 7 tags, got %d", len(tags))
	}

	// Check first tag (production environment)
	tag1 := tags[0].(map[string]interface{})
	if tag1["id"] != "tag-1" {
		t.Errorf("Expected id=tag-1, got %v", tag1["id"])
	}
	if tag1["name"] != "production" {
		t.Errorf("Expected name=production, got %v", tag1["name"])
	}
	if tag1["category"] != "environment" {
		t.Errorf("Expected category=environment, got %v", tag1["category"])
	}
	if tag1["color"] != "green" {
		t.Errorf("Expected color=green, got %v", tag1["color"])
	}
	if tag1["vm_count"].(float64) != 45 {
		t.Errorf("Expected vm_count=45, got %v", tag1["vm_count"])
	}

	// Verify categories are present
	categories := make(map[string]int)
	for _, t := range tags {
		tag := t.(map[string]interface{})
		cat := tag["category"].(string)
		categories[cat]++
	}

	if categories["environment"] != 3 {
		t.Errorf("Expected 3 environment tags, got %d", categories["environment"])
	}
	if categories["application"] != 2 {
		t.Errorf("Expected 2 application tags, got %d", categories["application"])
	}
	if categories["priority"] != 2 {
		t.Errorf("Expected 2 priority tags, got %d", categories["priority"])
	}
}

// Create Tag Handler Tests

func TestHandleCreateTagMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/tags", nil)
	w := httptest.NewRecorder()

	server.handleCreateTag(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateTagInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/tags",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateTag(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateTagValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := Tag{
		Name:     "testing",
		Category: "environment",
		Color:    "purple",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/tags",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateTag(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response Tag
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Name != "testing" {
		t.Errorf("Expected name=testing, got %s", response.Name)
	}
	if response.Category != "environment" {
		t.Errorf("Expected category=environment, got %s", response.Category)
	}
	if response.Color != "purple" {
		t.Errorf("Expected color=purple, got %s", response.Color)
	}
	if response.VMCount != 0 {
		t.Errorf("Expected vm_count=0 for new tag, got %d", response.VMCount)
	}
	if response.ID == "" {
		t.Error("Expected ID to be auto-generated")
	}
	if !strings.HasPrefix(response.ID, "tag-") {
		t.Errorf("Expected ID to start with 'tag-', got %s", response.ID)
	}
}

func TestHandleCreateTagDifferentCategories(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name     string
		tag      Tag
		category string
		color    string
	}{
		{
			"EnvironmentTag",
			Tag{Name: "qa", Category: "environment", Color: "cyan"},
			"environment",
			"cyan",
		},
		{
			"ApplicationTag",
			Tag{Name: "api", Category: "application", Color: "magenta"},
			"application",
			"magenta",
		},
		{
			"PriorityTag",
			Tag{Name: "low", Category: "priority", Color: "gray"},
			"priority",
			"gray",
		},
		{
			"CustomTag",
			Tag{Name: "backup", Category: "custom", Color: "brown"},
			"custom",
			"brown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.tag)
			req := httptest.NewRequest(http.MethodPost, "/tags",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleCreateTag(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("Expected status 201, got %d", w.Code)
			}

			var response Tag
			_ = json.Unmarshal(w.Body.Bytes(), &response)

			if response.Category != tt.category {
				t.Errorf("Expected category=%s, got %s", tt.category, response.Category)
			}
			if response.Color != tt.color {
				t.Errorf("Expected color=%s, got %s", tt.color, response.Color)
			}
		})
	}
}

// List Collections Handler Tests

func TestHandleListCollectionsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/collections", nil)
	w := httptest.NewRecorder()

	server.handleListCollections(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListCollections(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/collections", nil)
	w := httptest.NewRecorder()

	server.handleListCollections(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["collections"]; !ok {
		t.Error("Expected collections field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 2 {
		t.Errorf("Expected total=2, got %v", total)
	}

	// Verify collection structure
	collections := response["collections"].([]interface{})
	if len(collections) != 2 {
		t.Errorf("Expected 2 collections, got %d", len(collections))
	}

	// Check first collection
	coll1 := collections[0].(map[string]interface{})
	if coll1["id"] != "coll-1" {
		t.Errorf("Expected id=coll-1, got %v", coll1["id"])
	}
	if coll1["name"] != "Production Web Servers" {
		t.Errorf("Expected name='Production Web Servers', got %v", coll1["name"])
	}
	if coll1["description"] != "All production web application servers" {
		t.Errorf("Expected description='All production web application servers', got %v", coll1["description"])
	}
	if coll1["vm_count"].(float64) != 12 {
		t.Errorf("Expected vm_count=12, got %v", coll1["vm_count"])
	}
	if _, ok := coll1["created_at"]; !ok {
		t.Error("Expected created_at field")
	}

	// Check second collection
	coll2 := collections[1].(map[string]interface{})
	if coll2["name"] != "Database Cluster" {
		t.Errorf("Expected name='Database Cluster', got %v", coll2["name"])
	}
	if coll2["vm_count"].(float64) != 5 {
		t.Errorf("Expected vm_count=5, got %v", coll2["vm_count"])
	}
}

// Create Collection Handler Tests

func TestHandleCreateCollectionMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/collections", nil)
	w := httptest.NewRecorder()

	server.handleCreateCollection(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateCollectionInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/collections",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateCollection(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateCollectionValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := Collection{
		Name:        "Test Collection",
		Description: "Collection for testing",
		VMIDs:       []string{"vm-1", "vm-2", "vm-3"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/collections",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateCollection(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response Collection
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Name != "Test Collection" {
		t.Errorf("Expected name='Test Collection', got %s", response.Name)
	}
	if response.Description != "Collection for testing" {
		t.Errorf("Expected description='Collection for testing', got %s", response.Description)
	}
	if response.VMCount != 3 {
		t.Errorf("Expected vm_count=3, got %d", response.VMCount)
	}
	if len(response.VMIDs) != 3 {
		t.Errorf("Expected 3 VM IDs, got %d", len(response.VMIDs))
	}
	if response.ID == "" {
		t.Error("Expected ID to be auto-generated")
	}
	if !strings.HasPrefix(response.ID, "coll-") {
		t.Errorf("Expected ID to start with 'coll-', got %s", response.ID)
	}
	if response.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestHandleCreateCollectionDifferentSizes(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name        string
		collection  Collection
		expectedVMs int
	}{
		{
			"EmptyCollection",
			Collection{
				Name:        "Empty",
				Description: "No VMs",
				VMIDs:       []string{},
			},
			0,
		},
		{
			"SingleVM",
			Collection{
				Name:        "Single",
				Description: "One VM",
				VMIDs:       []string{"vm-1"},
			},
			1,
		},
		{
			"MultipleVMs",
			Collection{
				Name:        "Multiple",
				Description: "Many VMs",
				VMIDs:       []string{"vm-1", "vm-2", "vm-3", "vm-4", "vm-5"},
			},
			5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.collection)
			req := httptest.NewRequest(http.MethodPost, "/collections",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleCreateCollection(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("Expected status 201, got %d", w.Code)
			}

			var response Collection
			_ = json.Unmarshal(w.Body.Bytes(), &response)

			if response.VMCount != tt.expectedVMs {
				t.Errorf("Expected vm_count=%d, got %d", tt.expectedVMs, response.VMCount)
			}
		})
	}
}

// List Saved Searches Handler Tests

func TestHandleListSavedSearchesMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/searches", nil)
	w := httptest.NewRecorder()

	server.handleListSavedSearches(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListSavedSearches(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/searches", nil)
	w := httptest.NewRecorder()

	server.handleListSavedSearches(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["searches"]; !ok {
		t.Error("Expected searches field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 2 {
		t.Errorf("Expected total=2, got %v", total)
	}

	// Verify search structure
	searches := response["searches"].([]interface{})
	if len(searches) != 2 {
		t.Errorf("Expected 2 searches, got %d", len(searches))
	}

	// Check first search
	search1 := searches[0].(map[string]interface{})
	if search1["id"] != "search-1" {
		t.Errorf("Expected id=search-1, got %v", search1["id"])
	}
	if search1["name"] != "Production Windows VMs" {
		t.Errorf("Expected name='Production Windows VMs', got %v", search1["name"])
	}
	if search1["query"] != "tags:production AND os:windows" {
		t.Errorf("Expected query='tags:production AND os:windows', got %v", search1["query"])
	}
	if search1["results"].(float64) != 23 {
		t.Errorf("Expected results=23, got %v", search1["results"])
	}
	if _, ok := search1["created_at"]; !ok {
		t.Error("Expected created_at field")
	}

	// Check second search
	search2 := searches[1].(map[string]interface{})
	if search2["name"] != "High CPU Usage" {
		t.Errorf("Expected name='High CPU Usage', got %v", search2["name"])
	}
	if search2["query"] != "cpu:>80%" {
		t.Errorf("Expected query='cpu:>80%%', got %v", search2["query"])
	}
}

// Create Saved Search Handler Tests

func TestHandleCreateSavedSearchMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/searches", nil)
	w := httptest.NewRecorder()

	server.handleCreateSavedSearch(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateSavedSearchInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/searches",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateSavedSearch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateSavedSearchValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := SavedSearch{
		Name:    "My Search",
		Query:   "tags:production AND cpu:>50%",
		Results: 15,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/searches",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateSavedSearch(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response SavedSearch
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Name != "My Search" {
		t.Errorf("Expected name='My Search', got %s", response.Name)
	}
	if response.Query != "tags:production AND cpu:>50%" {
		t.Errorf("Expected query='tags:production AND cpu:>50%%', got %s", response.Query)
	}
	if response.Results != 15 {
		t.Errorf("Expected results=15, got %d", response.Results)
	}
	if response.ID == "" {
		t.Error("Expected ID to be auto-generated")
	}
	if !strings.HasPrefix(response.ID, "search-") {
		t.Errorf("Expected ID to start with 'search-', got %s", response.ID)
	}
	if response.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestHandleCreateSavedSearchDifferentQueries(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name   string
		search SavedSearch
		query  string
	}{
		{
			"TagSearch",
			SavedSearch{
				Name:    "Production VMs",
				Query:   "tags:production",
				Results: 45,
			},
			"tags:production",
		},
		{
			"OSSearch",
			SavedSearch{
				Name:    "Linux Servers",
				Query:   "os:linux",
				Results: 78,
			},
			"os:linux",
		},
		{
			"ComplexSearch",
			SavedSearch{
				Name:    "Critical Production",
				Query:   "tags:production AND tags:critical AND memory:>16GB",
				Results: 12,
			},
			"tags:production AND tags:critical AND memory:>16GB",
		},
		{
			"RegexSearch",
			SavedSearch{
				Name:    "Web Servers",
				Query:   "name:^web-.*",
				Results: 23,
			},
			"name:^web-.*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.search)
			req := httptest.NewRequest(http.MethodPost, "/searches",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleCreateSavedSearch(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("Expected status 201, got %d", w.Code)
			}

			var response SavedSearch
			_ = json.Unmarshal(w.Body.Bytes(), &response)

			if response.Query != tt.query {
				t.Errorf("Expected query=%s, got %s", tt.query, response.Query)
			}
		})
	}
}
