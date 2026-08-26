// SPDX-License-Identifier: Apache-2.0

// Package openapi provides OpenAPI/Swagger documentation support
package openapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Config holds OpenAPI configuration
type Config struct {
	// Enabled determines if OpenAPI is enabled
	Enabled bool

	// Title is the API title
	Title string

	// Description is the API description
	Description string

	// Version is the API version
	Version string

	// ServerURL is the server URL
	ServerURL string

	// Contact information
	ContactName  string
	ContactEmail string
	ContactURL   string

	// License information
	LicenseName string
	LicenseURL  string

	// SwaggerUIPath is the path to serve Swagger UI
	SwaggerUIPath string

	// SpecPath is the path to serve OpenAPI spec
	SpecPath string
}

// DefaultConfig returns default OpenAPI configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:       true,
		Title:         "HyperSDK API",
		Description:   "VM migration and conversion API for VMware to KVM",
		Version:       "1.0.0",
		ServerURL:     "http://localhost:8080",
		ContactName:   "HyperSDK Team",
		ContactEmail:  "support@example.com",
		LicenseName:   "Apache-2.0",
		LicenseURL:    "https://www.apache.org/licenses/LICENSE-2.0",
		SwaggerUIPath: "/api/docs",
		SpecPath:      "/api/openapi.json",
	}
}

// Generator generates OpenAPI specifications
type Generator struct {
	config *Config
	spec   *openapi3.T
}

// NewGenerator creates a new OpenAPI generator
func NewGenerator(config *Config) *Generator {
	if config == nil {
		config = DefaultConfig()
	}

	spec := &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:       config.Title,
			Description: config.Description,
			Version:     config.Version,
			Contact: &openapi3.Contact{
				Name:  config.ContactName,
				Email: config.ContactEmail,
				URL:   config.ContactURL,
			},
			License: &openapi3.License{
				Name: config.LicenseName,
				URL:  config.LicenseURL,
			},
		},
		Servers: openapi3.Servers{
			{
				URL:         config.ServerURL,
				Description: "HyperSDK API Server",
			},
		},
		Paths:      openapi3.NewPaths(),
		Components: &openapi3.Components{},
	}

	return &Generator{
		config: config,
		spec:   spec,
	}
}

// Generate generates the complete OpenAPI specification
func (g *Generator) Generate() *openapi3.T {
	g.addSecuritySchemes()
	g.addSchemas()
	g.addPaths()
	g.addTags()
	return g.spec
}

// addSecuritySchemes adds security scheme definitions
func (g *Generator) addSecuritySchemes() {
	if g.spec.Components.SecuritySchemes == nil {
		g.spec.Components.SecuritySchemes = make(openapi3.SecuritySchemes)
	}

	// Bearer token authentication
	g.spec.Components.SecuritySchemes["bearerAuth"] = &openapi3.SecuritySchemeRef{
		Value: &openapi3.SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "Bearer token authentication",
		},
	}

	// API key authentication
	g.spec.Components.SecuritySchemes["apiKey"] = &openapi3.SecuritySchemeRef{
		Value: &openapi3.SecurityScheme{
			Type:        "apiKey",
			In:          "header",
			Name:        "X-API-Key",
			Description: "API key authentication",
		},
	}
}

// addSchemas adds component schemas
func (g *Generator) addSchemas() {
	if g.spec.Components.Schemas == nil {
		g.spec.Components.Schemas = make(openapi3.Schemas)
	}

	// Job schema
	g.spec.Components.Schemas["Job"] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"id": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Job ID",
						Example:     "job-12345",
					},
				},
				"name": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Job name",
						Example:     "export-vm-prod-01",
					},
				},
				"status": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Job status",
						Enum:        []interface{}{"pending", "running", "completed", "failed", "cancelled"},
						Example:     "running",
					},
				},
				"progress": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"integer"},
						Description: "Progress percentage (0-100)",
						Min:         float64Ptr(0),
						Max:         float64Ptr(100),
						Example:     75,
					},
				},
				"vm_name": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Virtual machine name",
						Example:     "prod-web-01",
					},
				},
				"created_at": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Format:      "date-time",
						Description: "Creation timestamp",
					},
				},
				"updated_at": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Format:      "date-time",
						Description: "Last update timestamp",
					},
				},
			},
			Required: []string{"id", "name", "status"},
		},
	}

	// VM schema
	g.spec.Components.Schemas["VM"] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"name": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "VM name",
						Example:     "prod-db-01",
					},
				},
				"path": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "VM path on vCenter",
						Example:     "/Datacenter/vm/Production/prod-db-01",
					},
				},
				"power_state": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "VM power state",
						Enum:        []interface{}{"poweredOn", "poweredOff", "suspended"},
						Example:     "poweredOn",
					},
				},
				"cpu": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"integer"},
						Description: "Number of CPUs",
						Example:     4,
					},
				},
				"memory_mb": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"integer"},
						Description: "Memory in MB",
						Example:     8192,
					},
				},
			},
			Required: []string{"name", "path"},
		},
	}

	// Error schema
	g.spec.Components.Schemas["Error"] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"error": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Error message",
						Example:     "VM not found",
					},
				},
				"code": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Error code",
						Example:     "VM_NOT_FOUND",
					},
				},
				"details": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"object"},
						Description: "Additional error details",
					},
				},
			},
			Required: []string{"error"},
		},
	}

	// JobSubmitRequest schema
	g.spec.Components.Schemas["JobSubmitRequest"] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: openapi3.Schemas{
				"vm_name": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "VM name to export",
						Example:     "prod-web-01",
					},
				},
				"output_path": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Output path for exported VM",
						Example:     "/exports/prod-web-01",
					},
				},
				"provider": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Cloud provider (aws, azure, gcp)",
						Enum:        []interface{}{"aws", "azure", "gcp"},
						Example:     "aws",
					},
				},
				"options": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"object"},
						Description: "Provider-specific options",
					},
				},
			},
			Required: []string{"vm_name", "output_path"},
		},
	}
}

// addPaths adds API paths
func (g *Generator) addPaths() {
	// POST /api/jobs - Submit job
	g.spec.Paths.Set("/api/jobs", &openapi3.PathItem{
		Post: &openapi3.Operation{
			Tags:        []string{"Jobs"},
			Summary:     "Submit a new job",
			Description: "Submit a new VM export job",
			OperationID: "submitJob",
			Security: &openapi3.SecurityRequirements{
				{"bearerAuth": []string{}},
			},
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Description: "Job submission request",
					Required:    true,
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{
							Schema: &openapi3.SchemaRef{
								Ref: "#/components/schemas/JobSubmitRequest",
							},
						},
					},
				},
			},
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(201, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("Job created"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Ref: "#/components/schemas/Job",
								},
							},
						},
					},
				}),
				openapi3.WithStatus(400, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("Bad request"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Ref: "#/components/schemas/Error",
								},
							},
						},
					},
				}),
				openapi3.WithStatus(401, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("Unauthorized"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Ref: "#/components/schemas/Error",
								},
							},
						},
					},
				}),
			),
		},
		Get: &openapi3.Operation{
			Tags:        []string{"Jobs"},
			Summary:     "List jobs",
			Description: "List all jobs with optional filtering",
			OperationID: "listJobs",
			Security: &openapi3.SecurityRequirements{
				{"bearerAuth": []string{}},
			},
			Parameters: openapi3.Parameters{
				{
					Value: &openapi3.Parameter{
						Name:        "status",
						In:          "query",
						Description: "Filter by status",
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
								Enum: []interface{}{"pending", "running", "completed", "failed", "cancelled"},
							},
						},
					},
				},
				{
					Value: &openapi3.Parameter{
						Name:        "limit",
						In:          "query",
						Description: "Maximum number of results",
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type:    &openapi3.Types{"integer"},
								Default: 100,
								Min:     float64Ptr(1),
								Max:     float64Ptr(1000),
							},
						},
					},
				},
			},
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(200, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("List of jobs"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: &openapi3.Types{"array"},
										Items: &openapi3.SchemaRef{
											Ref: "#/components/schemas/Job",
										},
									},
								},
							},
						},
					},
				}),
			),
		},
	})

	// GET /api/jobs/{id} - Get job
	g.spec.Paths.Set("/api/jobs/{id}", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Tags:        []string{"Jobs"},
			Summary:     "Get job details",
			Description: "Get details of a specific job",
			OperationID: "getJob",
			Security: &openapi3.SecurityRequirements{
				{"bearerAuth": []string{}},
			},
			Parameters: openapi3.Parameters{
				{
					Value: &openapi3.Parameter{
						Name:        "id",
						In:          "path",
						Description: "Job ID",
						Required:    true,
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
							},
						},
					},
				},
			},
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(200, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("Job details"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Ref: "#/components/schemas/Job",
								},
							},
						},
					},
				}),
				openapi3.WithStatus(404, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("Job not found"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Ref: "#/components/schemas/Error",
								},
							},
						},
					},
				}),
			),
		},
		Delete: &openapi3.Operation{
			Tags:        []string{"Jobs"},
			Summary:     "Cancel job",
			Description: "Cancel a running job",
			OperationID: "cancelJob",
			Security: &openapi3.SecurityRequirements{
				{"bearerAuth": []string{}},
			},
			Parameters: openapi3.Parameters{
				{
					Value: &openapi3.Parameter{
						Name:        "id",
						In:          "path",
						Description: "Job ID",
						Required:    true,
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
							},
						},
					},
				},
			},
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(200, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("Job cancelled"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Ref: "#/components/schemas/Job",
								},
							},
						},
					},
				}),
				openapi3.WithStatus(404, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("Job not found"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Ref: "#/components/schemas/Error",
								},
							},
						},
					},
				}),
			),
		},
	})

	// GET /api/vms - List VMs
	g.spec.Paths.Set("/api/vms", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Tags:        []string{"VMs"},
			Summary:     "List virtual machines",
			Description: "List all available virtual machines from vCenter",
			OperationID: "listVMs",
			Security: &openapi3.SecurityRequirements{
				{"bearerAuth": []string{}},
			},
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(200, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("List of VMs"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: &openapi3.Types{"array"},
										Items: &openapi3.SchemaRef{
											Ref: "#/components/schemas/VM",
										},
									},
								},
							},
						},
					},
				}),
			),
		},
	})

	// GET /health - Health check
	g.spec.Paths.Set("/health", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Tags:        []string{"System"},
			Summary:     "Health check",
			Description: "Check API health status",
			OperationID: "healthCheck",
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(200, &openapi3.ResponseRef{
					Value: &openapi3.Response{
						Description: strPtr("API is healthy"),
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{
									Value: &openapi3.Schema{
										Type: &openapi3.Types{"object"},
										Properties: openapi3.Schemas{
											"status": &openapi3.SchemaRef{
												Value: &openapi3.Schema{
													Type:    &openapi3.Types{"string"},
													Example: "healthy",
												},
											},
										},
									},
								},
							},
						},
					},
				}),
			),
		},
	})
}

// addTags adds tag definitions
func (g *Generator) addTags() {
	g.spec.Tags = openapi3.Tags{
		{
			Name:        "Jobs",
			Description: "Job management endpoints",
		},
		{
			Name:        "VMs",
			Description: "Virtual machine endpoints",
		},
		{
			Name:        "System",
			Description: "System endpoints",
		},
	}
}

// Handler returns an HTTP handler for OpenAPI spec
func (g *Generator) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		spec := g.Generate()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(spec); err != nil {
			log.Printf("openapi: failed to encode spec response: %v", err)
		}
	}
}

// SwaggerUIHandler returns an HTTP handler for Swagger UI
func SwaggerUIHandler(specPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		html := generateSwaggerUIHTML(specPath)
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(html)); err != nil {
			log.Printf("openapi: failed to write swagger UI response: %v", err)
		}
	}
}

// generateSwaggerUIHTML generates Swagger UI HTML
func generateSwaggerUIHTML(specPath string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>HyperSDK API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui.css">
    <style>
        html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin: 0; padding: 0; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: "%s",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
            window.ui = ui;
        };
    </script>
</body>
</html>`, specPath)
}

// Helper functions

func strPtr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}

// RegisterHandlers registers OpenAPI handlers
func RegisterHandlers(mux *http.ServeMux, config *Config) {
	if !config.Enabled {
		return
	}

	generator := NewGenerator(config)

	// Register OpenAPI spec endpoint
	mux.HandleFunc(config.SpecPath, generator.Handler())

	// Register Swagger UI endpoint
	mux.HandleFunc(config.SwaggerUIPath, SwaggerUIHandler(config.SpecPath))

	// Redirect root docs path
	docsRoot := strings.TrimSuffix(config.SwaggerUIPath, "/")
	if docsRoot != config.SwaggerUIPath {
		mux.HandleFunc(docsRoot, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, config.SwaggerUIPath, http.StatusMovedPermanently)
		})
	}
}
