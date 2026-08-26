// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// Tag represents a VM tag
type Tag struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Color    string `json:"color"`
	VMCount  int    `json:"vm_count"`
}

// Collection represents a VM collection
type Collection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	VMIDs       []string  `json:"vm_ids"`
	VMCount     int       `json:"vm_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// SavedSearch represents a saved search query
type SavedSearch struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Query     string    `json:"query"`
	Results   int       `json:"results"`
	CreatedAt time.Time `json:"created_at"`
}

// handleListTags lists all tags
func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tags := []Tag{
		{ID: "tag-1", Name: "production", Category: "environment", Color: "green", VMCount: 45},
		{ID: "tag-2", Name: "development", Category: "environment", Color: "blue", VMCount: 23},
		{ID: "tag-3", Name: "staging", Category: "environment", Color: "yellow", VMCount: 12},
		{ID: "tag-4", Name: "web", Category: "application", Color: "purple", VMCount: 18},
		{ID: "tag-5", Name: "database", Category: "application", Color: "red", VMCount: 8},
		{ID: "tag-6", Name: "critical", Category: "priority", Color: "red", VMCount: 15},
		{ID: "tag-7", Name: "high", Category: "priority", Color: "orange", VMCount: 25},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"tags":  tags,
		"total": len(tags),
	})
}

// handleCreateTag creates a new tag
func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var tag Tag
	if err := json.NewDecoder(r.Body).Decode(&tag); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	tag.ID = "tag-" + time.Now().Format("20060102150405")
	tag.VMCount = 0

	s.jsonResponse(w, http.StatusCreated, tag)
}

// handleListCollections lists all collections
func (s *Server) handleListCollections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	collections := []Collection{
		{
			ID:          "coll-1",
			Name:        "Production Web Servers",
			Description: "All production web application servers",
			VMCount:     12,
			CreatedAt:   time.Now().Add(-10 * 24 * time.Hour),
		},
		{
			ID:          "coll-2",
			Name:        "Database Cluster",
			Description: "PostgreSQL database cluster nodes",
			VMCount:     5,
			CreatedAt:   time.Now().Add(-5 * 24 * time.Hour),
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"collections": collections,
		"total":       len(collections),
	})
}

// handleCreateCollection creates a new collection
func (s *Server) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var coll Collection
	if err := json.NewDecoder(r.Body).Decode(&coll); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	coll.ID = "coll-" + time.Now().Format("20060102150405")
	coll.CreatedAt = time.Now()
	coll.VMCount = len(coll.VMIDs)

	s.jsonResponse(w, http.StatusCreated, coll)
}

// handleListSavedSearches lists all saved searches
func (s *Server) handleListSavedSearches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	searches := []SavedSearch{
		{
			ID:        "search-1",
			Name:      "Production Windows VMs",
			Query:     "tags:production AND os:windows",
			Results:   23,
			CreatedAt: time.Now().Add(-7 * 24 * time.Hour),
		},
		{
			ID:        "search-2",
			Name:      "High CPU Usage",
			Query:     "cpu:>80%",
			Results:   5,
			CreatedAt: time.Now().Add(-3 * 24 * time.Hour),
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"searches": searches,
		"total":    len(searches),
	})
}

// handleCreateSavedSearch creates a new saved search
func (s *Server) handleCreateSavedSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var search SavedSearch
	if err := json.NewDecoder(r.Body).Decode(&search); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	search.ID = "search-" + time.Now().Format("20060102150405")
	search.CreatedAt = time.Now()

	s.jsonResponse(w, http.StatusCreated, search)
}
