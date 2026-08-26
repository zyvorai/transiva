# OpenAPI/Swagger Documentation

This package provides comprehensive API documentation using OpenAPI 3.0 specification with an integrated Swagger UI for interactive testing.

## Features

- **OpenAPI 3.0 Specification**: Industry-standard API documentation
- **Swagger UI Integration**: Interactive API testing interface
- **Automatic Schema Generation**: Type-safe request/response schemas
- **Security Schemes**: Bearer token and API key authentication
- **Comprehensive Coverage**: All API endpoints documented
- **Examples**: Request/response examples for all operations

## Quick Start

### Basic Setup

```go
import (
    "net/http"
    "github.com/zyvorai/transiva/daemon/openapi"
)

func main() {
    mux := http.NewServeMux()

    // Use default configuration
    config := openapi.DefaultConfig()
    config.ServerURL = "http://localhost:8080"

    // Register OpenAPI handlers
    openapi.RegisterHandlers(mux, config)

    // Start server
    http.ListenAndServe(":8080", mux)
}
```

### Access Documentation

Once the server is running:

- **Swagger UI**: http://localhost:8080/api/docs
- **OpenAPI Spec**: http://localhost:8080/api/openapi.json

## Configuration

```go
config := &openapi.Config{
    Enabled:       true,
    Title:         "HyperSDK API",
    Description:   "VM migration and conversion API",
    Version:       "1.0.0",
    ServerURL:     "http://localhost:8080",

    // Contact information
    ContactName:   "HyperSDK Team",
    ContactEmail:  "support@example.com",
    ContactURL:    "https://example.com/support",

    // License
    LicenseName:   "Apache-2.0",
    LicenseURL:    "https://www.apache.org/licenses/LICENSE-2.0",

    // Paths
    SwaggerUIPath: "/api/docs",
    SpecPath:      "/api/openapi.json",
}
```

## API Endpoints Documented

### Jobs

- `POST /api/jobs` - Submit a new VM export job
- `GET /api/jobs` - List all jobs with filtering
- `GET /api/jobs/{id}` - Get job details
- `DELETE /api/jobs/{id}` - Cancel a running job

### Virtual Machines

- `GET /api/vms/list?provider={vsphere|nutanix}` - List VMs from a provider
- `GET /api/vms/info?provider={vsphere|nutanix}&vm={name|path}` - VM details
- `POST /api/vms/export?provider=nutanix` - Synchronous Nutanix NFS pickup export
- `GET /api/providers/list` - Registered providers
- `GET /api/providers/capabilities?provider={name}` - Provider export capabilities

See [docs/nutanix.md](../../docs/nutanix.md) for Nutanix request bodies and mount configuration.

### System

- `GET /health` - API health check

## Request/Response Schemas

### Job Schema

```json
{
  "id": "job-12345",
  "name": "export-vm-prod-01",
  "status": "running",
  "progress": 75,
  "vm_name": "prod-web-01",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:35:00Z"
}
```

**Status values**: `pending`, `running`, `completed`, `failed`, `cancelled`

### VM Schema

```json
{
  "name": "prod-db-01",
  "path": "/Datacenter/vm/Production/prod-db-01",
  "power_state": "poweredOn",
  "cpu": 4,
  "memory_mb": 8192
}
```

### Job Submit Request

```json
{
  "vm_name": "prod-web-01",
  "output_path": "/exports/prod-web-01",
  "provider": "aws",
  "options": {
    "region": "us-east-1",
    "instance_type": "t3.medium"
  }
}
```

**Supported providers**: `vsphere`, `nutanix`, `aws`, `azure`, `gcp`

### Error Response

```json
{
  "error": "VM not found",
  "code": "VM_NOT_FOUND",
  "details": {
    "vm_name": "nonexistent-vm"
  }
}
```

## Authentication

The API supports two authentication methods:

### Bearer Token (Recommended)

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
     http://localhost:8080/api/jobs
```

### API Key

```bash
curl -H "X-API-Key: YOUR_API_KEY" \
     http://localhost:8080/api/jobs
```

## Using Swagger UI

### Interactive Testing

1. Navigate to http://localhost:8080/api/docs
2. Click "Authorize" button
3. Enter your bearer token or API key
4. Click "Authorize" to save credentials
5. Expand any endpoint and click "Try it out"
6. Fill in parameters and click "Execute"
7. View the response

### Example: Submit a Job

1. Expand `POST /api/jobs`
2. Click "Try it out"
3. Modify the request body:
   ```json
   {
     "vm_name": "my-vm",
     "output_path": "/exports/my-vm",
     "provider": "aws"
   }
   ```
4. Click "Execute"
5. View the response with job details

## Customization

### Adding Custom Endpoints

```go
generator := openapi.NewGenerator(config)

// Add custom path
customPath := &openapi3.PathItem{
    Get: &openapi3.Operation{
        Tags:        []string{"Custom"},
        Summary:     "Custom endpoint",
        Description: "My custom endpoint",
        OperationID: "customOp",
        Responses: openapi3.NewResponses(
            openapi3.WithStatus(200, &openapi3.Response{
                Description: strPtr("Success"),
            }),
        ),
    },
}

spec := generator.Generate()
spec.Paths.Set("/api/custom", customPath)
```

### Adding Custom Schemas

```go
generator := openapi.NewGenerator(config)
spec := generator.Generate()

spec.Components.Schemas["CustomModel"] = &openapi3.SchemaRef{
    Value: &openapi3.Schema{
        Type: "object",
        Properties: openapi3.Schemas{
            "field1": &openapi3.SchemaRef{
                Value: &openapi3.Schema{
                    Type:        "string",
                    Description: "Field description",
                },
            },
        },
        Required: []string{"field1"},
    },
}
```

## Advanced Configuration

### Multiple Environments

```go
config := openapi.DefaultConfig()
config.Servers = openapi3.Servers{
    {
        URL:         "http://localhost:8080",
        Description: "Development",
    },
    {
        URL:         "https://staging.example.com",
        Description: "Staging",
    },
    {
        URL:         "https://api.example.com",
        Description: "Production",
    },
}
```

### Custom Security Schemes

```go
generator := openapi.NewGenerator(config)
spec := generator.Generate()

spec.Components.SecuritySchemes["oauth2"] = &openapi3.SecuritySchemeRef{
    Value: &openapi3.SecurityScheme{
        Type: "oauth2",
        Flows: &openapi3.OAuthFlows{
            AuthorizationCode: &openapi3.OAuthFlow{
                AuthorizationURL: "https://auth.example.com/authorize",
                TokenURL:         "https://auth.example.com/token",
                Scopes: map[string]string{
                    "read:jobs":  "Read jobs",
                    "write:jobs": "Write jobs",
                },
            },
        },
    },
}
```

## Integration with Existing Server

```go
import (
    "github.com/zyvorai/transiva/daemon/openapi"
    "github.com/zyvorai/transiva/daemon/server"
)

func setupServer() {
    mux := http.NewServeMux()

    // Register your API handlers
    mux.HandleFunc("/api/jobs", handleJobs)
    mux.HandleFunc("/api/jobs/{id}", handleJob)
    mux.HandleFunc("/api/vms", handleVMs)

    // Add OpenAPI documentation
    openapiConfig := openapi.DefaultConfig()
    openapi.RegisterHandlers(mux, openapiConfig)

    http.ListenAndServe(":8080", mux)
}
```

## Validation

### Validate Requests

```go
import (
    "github.com/getkin/kin-openapi/openapi3filter"
    "github.com/getkin/kin-openapi/routers/gorillamux"
)

func validateRequest(r *http.Request, spec *openapi3.T) error {
    router, err := gorillamux.NewRouter(spec)
    if err != nil {
        return err
    }

    route, pathParams, err := router.FindRoute(r)
    if err != nil {
        return err
    }

    requestValidationInput := &openapi3filter.RequestValidationInput{
        Request:    r,
        PathParams: pathParams,
        Route:      route,
    }

    return openapi3filter.ValidateRequest(r.Context(), requestValidationInput)
}
```

## Client SDK Generation

### Generate Go Client

```bash
# Install openapi-generator
npm install -g @openapitools/openapi-generator-cli

# Generate Go client
openapi-generator-cli generate \
    -i http://localhost:8080/api/openapi.json \
    -g go \
    -o ./client
```

### Generate Python Client

```bash
openapi-generator-cli generate \
    -i http://localhost:8080/api/openapi.json \
    -g python \
    -o ./python-client
```

### Generate TypeScript Client

```bash
openapi-generator-cli generate \
    -i http://localhost:8080/api/openapi.json \
    -g typescript-axios \
    -o ./ts-client
```

## Testing

```bash
# Run tests
go test ./daemon/openapi/...

# Run tests with coverage
go test ./daemon/openapi/... -cover

# View coverage report
go test ./daemon/openapi/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Best Practices

1. **Keep Documentation Updated**: Update OpenAPI spec when adding new endpoints
2. **Use Meaningful Descriptions**: Provide clear descriptions for operations and parameters
3. **Include Examples**: Add request/response examples for better understanding
4. **Version Your API**: Use semantic versioning for API versions
5. **Document Error Responses**: Include all possible error responses
6. **Use Tags**: Organize endpoints with meaningful tags
7. **Validate Schemas**: Use schema validation for requests/responses

## Troubleshooting

### Swagger UI Not Loading

1. Check that the server is running
2. Verify the spec path is accessible: `curl http://localhost:8080/api/openapi.json`
3. Check browser console for errors
4. Ensure CORS is configured if accessing from different origin

### Invalid OpenAPI Spec

```bash
# Validate spec using openapi-generator
openapi-generator-cli validate -i http://localhost:8080/api/openapi.json

# Or use online validator
# https://validator.swagger.io/
```

### Missing Endpoints in Documentation

Ensure endpoints are added to the spec:

```go
generator := openapi.NewGenerator(config)
spec := generator.Generate()

// Verify paths
for path := range spec.Paths.Map() {
    fmt.Printf("Path: %s\n", path)
}
```

## Performance Considerations

- OpenAPI spec is generated on each request - consider caching
- Swagger UI loads external assets - host locally for production
- Validation adds overhead - use selectively

### Caching Example

```go
var cachedSpec *openapi3.T
var specMutex sync.RWMutex

func getCachedSpec(generator *openapi.Generator) *openapi3.T {
    specMutex.RLock()
    if cachedSpec != nil {
        defer specMutex.RUnlock()
        return cachedSpec
    }
    specMutex.RUnlock()

    specMutex.Lock()
    defer specMutex.Unlock()

    cachedSpec = generator.Generate()
    return cachedSpec
}
```

## Security Considerations

1. **Authentication Required**: Require authentication for sensitive endpoints
2. **Rate Limiting**: Apply rate limits to prevent abuse
3. **Input Validation**: Validate all inputs against schema
4. **HTTPS in Production**: Always use HTTPS in production
5. **API Keys**: Rotate API keys regularly
6. **CORS Configuration**: Configure CORS appropriately

## Resources

- [OpenAPI Specification](https://swagger.io/specification/)
- [Swagger UI Documentation](https://swagger.io/tools/swagger-ui/)
- [kin-openapi Library](https://github.com/getkin/kin-openapi)
- [OpenAPI Generator](https://openapi-generator.tech/)

## License

SPDX-License-Identifier: Apache-2.0
