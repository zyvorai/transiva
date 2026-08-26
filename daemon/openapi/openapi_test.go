// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("expected OpenAPI to be enabled by default")
	}

	if config.Title != "HyperSDK API" {
		t.Errorf("expected title 'HyperSDK API', got %s", config.Title)
	}

	if config.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", config.Version)
	}

	if config.SwaggerUIPath != "/api/docs" {
		t.Errorf("expected Swagger UI path '/api/docs', got %s", config.SwaggerUIPath)
	}

	if config.SpecPath != "/api/openapi.json" {
		t.Errorf("expected spec path '/api/openapi.json', got %s", config.SpecPath)
	}
}

func TestNewGenerator(t *testing.T) {
	config := DefaultConfig()
	generator := NewGenerator(config)

	if generator == nil {
		t.Fatal("expected generator to be created")
	}

	if generator.config != config {
		t.Error("expected config to be set")
	}

	if generator.spec == nil {
		t.Error("expected spec to be initialized")
	}

	if generator.spec.OpenAPI != "3.0.0" {
		t.Errorf("expected OpenAPI version 3.0.0, got %s", generator.spec.OpenAPI)
	}
}

func TestNewGeneratorNilConfig(t *testing.T) {
	generator := NewGenerator(nil)

	if generator == nil {
		t.Fatal("expected generator to be created with default config")
	}

	if generator.config == nil {
		t.Error("expected default config to be set")
	}
}

func TestGenerate(t *testing.T) {
	config := DefaultConfig()
	generator := NewGenerator(config)
	spec := generator.Generate()

	if spec == nil {
		t.Fatal("expected spec to be generated")
	}

	// Verify info
	if spec.Info == nil {
		t.Fatal("expected info to be set")
	}

	if spec.Info.Title != config.Title {
		t.Errorf("expected title %s, got %s", config.Title, spec.Info.Title)
	}

	if spec.Info.Version != config.Version {
		t.Errorf("expected version %s, got %s", config.Version, spec.Info.Version)
	}

	// Verify servers
	if len(spec.Servers) == 0 {
		t.Error("expected at least one server")
	}

	if spec.Servers[0].URL != config.ServerURL {
		t.Errorf("expected server URL %s, got %s", config.ServerURL, spec.Servers[0].URL)
	}

	// Verify components
	if spec.Components == nil {
		t.Fatal("expected components to be set")
	}

	// Verify security schemes
	if spec.Components.SecuritySchemes == nil {
		t.Fatal("expected security schemes to be set")
	}

	if _, ok := spec.Components.SecuritySchemes["bearerAuth"]; !ok {
		t.Error("expected bearerAuth security scheme")
	}

	if _, ok := spec.Components.SecuritySchemes["apiKey"]; !ok {
		t.Error("expected apiKey security scheme")
	}

	// Verify schemas
	if spec.Components.Schemas == nil {
		t.Fatal("expected schemas to be set")
	}

	expectedSchemas := []string{"Job", "VM", "Error", "JobSubmitRequest"}
	for _, schemaName := range expectedSchemas {
		if _, ok := spec.Components.Schemas[schemaName]; !ok {
			t.Errorf("expected schema %s", schemaName)
		}
	}

	// Verify paths
	if spec.Paths == nil {
		t.Fatal("expected paths to be set")
	}

	expectedPaths := []string{"/api/jobs", "/api/jobs/{id}", "/api/vms", "/health"}
	for _, path := range expectedPaths {
		if spec.Paths.Find(path) == nil {
			t.Errorf("expected path %s", path)
		}
	}

	// Verify tags
	if len(spec.Tags) == 0 {
		t.Error("expected tags to be set")
	}

	expectedTags := []string{"Jobs", "VMs", "System"}
	for _, tagName := range expectedTags {
		found := false
		for _, tag := range spec.Tags {
			if tag.Name == tagName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tag %s", tagName)
		}
	}
}

func TestJobSchema(t *testing.T) {
	generator := NewGenerator(DefaultConfig())
	spec := generator.Generate()

	jobSchema := spec.Components.Schemas["Job"]
	if jobSchema == nil {
		t.Fatal("expected Job schema")
	}

	schema := jobSchema.Value
	if schema == nil {
		t.Fatal("expected schema value")
	}

	// Verify required fields
	expectedRequired := []string{"id", "name", "status"}
	if len(schema.Required) != len(expectedRequired) {
		t.Errorf("expected %d required fields, got %d", len(expectedRequired), len(schema.Required))
	}

	// Verify properties
	expectedProperties := []string{"id", "name", "status", "progress", "vm_name", "created_at", "updated_at"}
	for _, prop := range expectedProperties {
		if _, ok := schema.Properties[prop]; !ok {
			t.Errorf("expected property %s", prop)
		}
	}

	// Verify status enum
	statusProp := schema.Properties["status"]
	if statusProp == nil {
		t.Fatal("expected status property")
	}

	if statusProp.Value.Enum == nil {
		t.Error("expected status to have enum values")
	}
}

func TestVMSchema(t *testing.T) {
	generator := NewGenerator(DefaultConfig())
	spec := generator.Generate()

	vmSchema := spec.Components.Schemas["VM"]
	if vmSchema == nil {
		t.Fatal("expected VM schema")
	}

	schema := vmSchema.Value
	if schema == nil {
		t.Fatal("expected schema value")
	}

	// Verify required fields
	expectedRequired := []string{"name", "path"}
	if len(schema.Required) != len(expectedRequired) {
		t.Errorf("expected %d required fields, got %d", len(expectedRequired), len(schema.Required))
	}

	// Verify properties
	expectedProperties := []string{"name", "path", "power_state", "cpu", "memory_mb"}
	for _, prop := range expectedProperties {
		if _, ok := schema.Properties[prop]; !ok {
			t.Errorf("expected property %s", prop)
		}
	}
}

func TestAPIEndpoints(t *testing.T) {
	generator := NewGenerator(DefaultConfig())
	spec := generator.Generate()

	// Test POST /api/jobs
	jobsPath := spec.Paths.Find("/api/jobs")
	if jobsPath == nil {
		t.Fatal("expected /api/jobs path")
	}

	if jobsPath.Post == nil {
		t.Error("expected POST operation on /api/jobs")
	}

	if jobsPath.Get == nil {
		t.Error("expected GET operation on /api/jobs")
	}

	// Verify POST operation
	postOp := jobsPath.Post
	if postOp.OperationID != "submitJob" {
		t.Errorf("expected operation ID 'submitJob', got %s", postOp.OperationID)
	}

	if postOp.RequestBody == nil {
		t.Error("expected request body")
	}

	if postOp.Responses == nil {
		t.Error("expected responses")
	}

	if postOp.Responses.Status(201) == nil {
		t.Error("expected 201 response")
	}

	if postOp.Responses.Status(400) == nil {
		t.Error("expected 400 response")
	}

	if postOp.Responses.Status(401) == nil {
		t.Error("expected 401 response")
	}

	// Test GET /api/jobs/{id}
	jobPath := spec.Paths.Find("/api/jobs/{id}")
	if jobPath == nil {
		t.Fatal("expected /api/jobs/{id} path")
	}

	if jobPath.Get == nil {
		t.Error("expected GET operation on /api/jobs/{id}")
	}

	if jobPath.Delete == nil {
		t.Error("expected DELETE operation on /api/jobs/{id}")
	}

	// Verify GET operation has id parameter
	getOp := jobPath.Get
	if len(getOp.Parameters) == 0 {
		t.Error("expected parameters")
	}

	idParam := getOp.Parameters[0].Value
	if idParam.Name != "id" {
		t.Errorf("expected parameter name 'id', got %s", idParam.Name)
	}

	if idParam.In != "path" {
		t.Errorf("expected parameter in 'path', got %s", idParam.In)
	}

	if !idParam.Required {
		t.Error("expected id parameter to be required")
	}
}

func TestHandler(t *testing.T) {
	config := DefaultConfig()
	generator := NewGenerator(config)
	handler := generator.Handler()

	req := httptest.NewRequest("GET", "/api/openapi.json", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected content type application/json, got %s", contentType)
	}

	// Verify valid JSON
	var spec openapi3.T
	err := json.NewDecoder(rr.Body).Decode(&spec)
	if err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	if spec.OpenAPI != "3.0.0" {
		t.Errorf("expected OpenAPI version 3.0.0, got %s", spec.OpenAPI)
	}
}

func TestSwaggerUIHandler(t *testing.T) {
	handler := SwaggerUIHandler("/api/openapi.json")

	req := httptest.NewRequest("GET", "/api/docs", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/html" {
		t.Errorf("expected content type text/html, got %s", contentType)
	}

	body := rr.Body.String()

	// Verify Swagger UI HTML
	if !containsString(body, "<!DOCTYPE html>") {
		t.Error("expected HTML document")
	}

	if !containsString(body, "swagger-ui") {
		t.Error("expected swagger-ui reference")
	}

	if !containsString(body, "/api/openapi.json") {
		t.Error("expected spec path in HTML")
	}

	if !containsString(body, "HyperSDK API Documentation") {
		t.Error("expected page title")
	}
}

func TestRegisterHandlers(t *testing.T) {
	mux := http.NewServeMux()
	config := DefaultConfig()

	RegisterHandlers(mux, config)

	// Test OpenAPI spec endpoint
	req := httptest.NewRequest("GET", config.SpecPath, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for spec endpoint, got %d", rr.Code)
	}

	// Test Swagger UI endpoint
	req = httptest.NewRequest("GET", config.SwaggerUIPath, nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for Swagger UI endpoint, got %d", rr.Code)
	}
}

func TestRegisterHandlersDisabled(t *testing.T) {
	mux := http.NewServeMux()
	config := DefaultConfig()
	config.Enabled = false

	RegisterHandlers(mux, config)

	// Test that handlers are not registered
	req := httptest.NewRequest("GET", config.SpecPath, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 when disabled, got %d", rr.Code)
	}
}

func TestSecuritySchemes(t *testing.T) {
	generator := NewGenerator(DefaultConfig())
	spec := generator.Generate()

	// Test bearer auth
	bearerAuth := spec.Components.SecuritySchemes["bearerAuth"]
	if bearerAuth == nil {
		t.Fatal("expected bearerAuth security scheme")
	}

	if bearerAuth.Value.Type != "http" {
		t.Errorf("expected type http, got %s", bearerAuth.Value.Type)
	}

	if bearerAuth.Value.Scheme != "bearer" {
		t.Errorf("expected scheme bearer, got %s", bearerAuth.Value.Scheme)
	}

	if bearerAuth.Value.BearerFormat != "JWT" {
		t.Errorf("expected bearer format JWT, got %s", bearerAuth.Value.BearerFormat)
	}

	// Test API key auth
	apiKey := spec.Components.SecuritySchemes["apiKey"]
	if apiKey == nil {
		t.Fatal("expected apiKey security scheme")
	}

	if apiKey.Value.Type != "apiKey" {
		t.Errorf("expected type apiKey, got %s", apiKey.Value.Type)
	}

	if apiKey.Value.In != "header" {
		t.Errorf("expected in header, got %s", apiKey.Value.In)
	}

	if apiKey.Value.Name != "X-API-Key" {
		t.Errorf("expected name X-API-Key, got %s", apiKey.Value.Name)
	}
}

func TestQueryParameters(t *testing.T) {
	generator := NewGenerator(DefaultConfig())
	spec := generator.Generate()

	// Test GET /api/jobs query parameters
	jobsPath := spec.Paths.Find("/api/jobs")
	if jobsPath == nil {
		t.Fatal("expected /api/jobs path")
	}

	getOp := jobsPath.Get
	if getOp == nil {
		t.Fatal("expected GET operation")
	}

	if len(getOp.Parameters) == 0 {
		t.Fatal("expected query parameters")
	}

	// Find status parameter
	var statusParam *openapi3.Parameter
	var limitParam *openapi3.Parameter

	for _, param := range getOp.Parameters {
		if param.Value.Name == "status" {
			statusParam = param.Value
		}
		if param.Value.Name == "limit" {
			limitParam = param.Value
		}
	}

	if statusParam == nil {
		t.Error("expected status query parameter")
	}

	if limitParam == nil {
		t.Error("expected limit query parameter")
	}

	// Verify limit parameter
	if limitParam != nil {
		if limitParam.Schema.Value.Type == nil || !limitParam.Schema.Value.Type.Is("integer") {
			t.Errorf("expected limit type integer, got %v", limitParam.Schema.Value.Type)
		}

		if limitParam.Schema.Value.Default != 100 {
			t.Errorf("expected limit default 100, got %v", limitParam.Schema.Value.Default)
		}
	}
}

func TestErrorSchema(t *testing.T) {
	generator := NewGenerator(DefaultConfig())
	spec := generator.Generate()

	errorSchema := spec.Components.Schemas["Error"]
	if errorSchema == nil {
		t.Fatal("expected Error schema")
	}

	schema := errorSchema.Value
	if schema == nil {
		t.Fatal("expected schema value")
	}

	// Verify required fields
	if len(schema.Required) != 1 || schema.Required[0] != "error" {
		t.Error("expected 'error' to be required")
	}

	// Verify properties
	expectedProperties := []string{"error", "code", "details"}
	for _, prop := range expectedProperties {
		if _, ok := schema.Properties[prop]; !ok {
			t.Errorf("expected property %s", prop)
		}
	}
}

// Helper functions

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
