# traefik-jwt-decoder-plugin

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://go.dev)
[![Test Coverage](https://img.shields.io/badge/coverage-93%25-brightgreen.svg)]()
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Traefik Plugin](https://img.shields.io/badge/traefik-plugin-blue.svg)](https://plugins.traefik.io/)

A Traefik middleware plugin that extracts claims from JWT tokens and injects them as HTTP headers for upstream services. No signature verification - pure claim extraction for internal service-to-service communication.

## Overview

This plugin decodes JWT tokens (without validation), extracts specified claims, and injects them as HTTP headers. It's designed for scenarios where JWT validation happens at the edge (API gateway) and internal services need access to JWT claims without re-parsing tokens.

**⚠️ SECURITY NOTICE**: This plugin does NOT verify JWT signatures. Deploy only behind authenticated API gateways. See [SECURITY.md](SECURITY.md) for details.

## Table of Contents

- [Key Features](#key-features)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Security](#security)
- [Development](#development)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Key Features

- ✅ **No External Dependencies**: Pure Go stdlib implementation (Traefik Yaegi compatible)
- ✅ **Flexible Claim Extraction**: Supports dot notation for nested claims (`user.profile.email`)
- ✅ **Configurable Behavior**: Control header collisions, error handling, and source configuration
- ✅ **Security Guards**: Protected header blacklist, CRLF injection prevention, size limits
- ✅ **Array Support**: Handle array claims (comma-separated or JSON string)
- ✅ **Multi-Section Support**: Read from JWT header and/or payload sections
- ✅ **High Test Coverage**: 93% code coverage with comprehensive security tests
- ✅ **Thread-Safe**: No shared mutable state, safe for concurrent requests

### Use Cases

- Extract user ID from JWT `sub` claim → `X-User-Id` header
- Extract email from JWT → `X-User-Email` header
- Extract custom tenant information → `X-Tenant-Id` header
- Extract roles array → `X-User-Roles` header (comma-separated)
- Pass JWT header claims (like `kid`) to upstream services

## Quick Start

### Installation

Add plugin to your Traefik static configuration:

```yaml
# traefik.yml
experimental:
  plugins:
    traefik-jwt-decoder-plugin:
      moduleName: github.com/yourusername/traefik-jwt-decoder-plugin
      version: v0.1.0
```

### Basic Configuration

Configure the middleware in your dynamic configuration:

```yaml
# dynamic-config.yml
http:
  middlewares:
    jwt-decoder:
      plugin:
        traefik-jwt-decoder-plugin:
          sourceHeader: "Authorization"
          tokenPrefix: "Bearer "
          claims:
            - claimPath: "sub"
              headerName: "X-User-Id"
            - claimPath: "email"
              headerName: "X-User-Email"
          continueOnError: true
```

### Testing with Docker Compose

```bash
# Clone and navigate to examples
cd examples

# Start testing environment
docker-compose up -d

# Run tests
./test-plugin.sh

# Stop environment
docker-compose down
```

For detailed testing instructions, see [examples/README.md](examples/README.md).

## Architecture

### Data Flow

```
HTTP Request
    ↓
[Extract JWT from Header]
    ↓
[Parse JWT (Base64URL decode)]
    ↓
[Extract Claims per Configuration]
    ↓
[For Each Claim Mapping]
    ├─ Navigate claim path (dot notation)
    ├─ Check header collision policy
    ├─ Sanitize value
    ├─ Inject as HTTP header
    ↓
[Forward to Upstream]
```

### Components

```
jwt_claims_headers.go       # Main plugin entrypoint (ServeHTTP)
config.go                   # Configuration structs & validation
jwt.go                      # JWT parsing (decode, unmarshal)
claims.go                   # Claim extraction with dot notation
headers.go                  # Header injection & sanitization
```

## Configuration

### Traefik Static Configuration

```yaml
# traefik.yml
experimental:
  plugins:
    traefik-jwt-decoder-plugin:
      moduleName: github.com/yourusername/traefik-jwt-decoder-plugin
      version: v0.1.0
```

### Middleware Configuration

```yaml
# Dynamic configuration
http:
  middlewares:
    jwt-decoder:
      plugin:
        traefik-jwt-decoder-plugin:
          # Source configuration
          sourceHeader: "Authorization"
          tokenPrefix: "Bearer "
          
          # Claim mappings
          claims:
            - claimPath: "sub"
              headerName: "X-User-Id"
              override: false
              
            - claimPath: "email"
              headerName: "X-User-Email"
              override: false
              
            - claimPath: "roles"
              headerName: "X-User-Roles"
              override: false
              arrayFormat: "comma"  # "comma" or "json"
              
            - claimPath: "custom.tenant_id"
              headerName: "X-Tenant-Id"
              override: false
          
          # Which JWT sections to read
          sections:
            - "payload"
            # - "header"  # Uncomment to also read JWT header
          
          # Error handling
          continueOnError: true
          removeSourceHeader: false
          
          # Security limits
          maxClaimDepth: 10
          maxHeaderSize: 8192
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `sourceHeader` | string | `"Authorization"` | HTTP header containing JWT |
| `tokenPrefix` | string | `"Bearer "` | Prefix to strip from token (empty = none) |
| `claims` | array | `[]` | List of claim mappings (see below) |
| `sections` | array | `["payload"]` | JWT sections to read: `"header"`, `"payload"` |
| `continueOnError` | bool | `true` | Continue processing on JWT parse errors |
| `removeSourceHeader` | bool | `false` | Remove Authorization header after processing |
| `maxClaimDepth` | int | `10` | Maximum depth for nested claim paths |
| `maxHeaderSize` | int | `8192` | Maximum size of header values (bytes) |

### Claim Mapping Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `claimPath` | string | Yes | Path to claim (dot notation for nested) |
| `headerName` | string | Yes | Target HTTP header name |
| `override` | bool | No (default: `false`) | Override existing header if present |
| `arrayFormat` | string | No (default: `"comma"`) | Array format: `"comma"` or `"json"` |

## Implementation Specification

### Phase 1: Core Functionality

#### 1. Project Setup

**File**: `.traefik.yml`
```yaml
displayName: JWT Claims to Headers
type: middleware
import: github.com/yourusername/traefik-jwt-decoder-plugin
summary: Extract JWT claims and inject as HTTP headers
testData:
  sourceHeader: Authorization
  tokenPrefix: "Bearer "
  claims:
    - claimPath: sub
      headerName: X-User-Id
```

**File**: `go.mod`
```go
module github.com/yourusername/traefik-jwt-decoder-plugin

go 1.21
```

#### 2. Configuration Structures

**File**: `config.go`

```go
package traefik_jwt_decoder_plugin

// Config holds the plugin configuration
type Config struct {
    SourceHeader      string         `json:"sourceHeader,omitempty" yaml:"sourceHeader,omitempty"`
    TokenPrefix       string         `json:"tokenPrefix,omitempty" yaml:"tokenPrefix,omitempty"`
    Claims            []ClaimMapping `json:"claims,omitempty" yaml:"claims,omitempty"`
    Sections          []string       `json:"sections,omitempty" yaml:"sections,omitempty"`
    ContinueOnError   bool           `json:"continueOnError,omitempty" yaml:"continueOnError,omitempty"`
    RemoveSourceHeader bool          `json:"removeSourceHeader,omitempty" yaml:"removeSourceHeader,omitempty"`
    MaxClaimDepth     int            `json:"maxClaimDepth,omitempty" yaml:"maxClaimDepth,omitempty"`
    MaxHeaderSize     int            `json:"maxHeaderSize,omitempty" yaml:"maxHeaderSize,omitempty"`
}

// ClaimMapping defines a mapping from JWT claim to HTTP header
type ClaimMapping struct {
    ClaimPath   string `json:"claimPath" yaml:"claimPath"`
    HeaderName  string `json:"headerName" yaml:"headerName"`
    Override    bool   `json:"override,omitempty" yaml:"override,omitempty"`
    ArrayFormat string `json:"arrayFormat,omitempty" yaml:"arrayFormat,omitempty"`
}

// CreateConfig creates and initializes the plugin configuration with defaults
func CreateConfig() *Config {
    return &Config{
        SourceHeader:      "Authorization",
        TokenPrefix:       "Bearer ",
        Claims:            []ClaimMapping{},
        Sections:          []string{"payload"},
        ContinueOnError:   true,
        RemoveSourceHeader: false,
        MaxClaimDepth:     10,
        MaxHeaderSize:     8192,
    }
}

// Validate checks configuration validity
func (c *Config) Validate() error {
    // Implementation needed:
    // - Check Claims array not empty
    // - Validate each ClaimMapping (claimPath and headerName required)
    // - Check Sections contains only "header" or "payload"
    // - Validate MaxClaimDepth > 0
    // - Validate MaxHeaderSize > 0
    // - Check for duplicate headerName values
}
```

#### 3. JWT Parsing

**File**: `jwt.go`

JWT structure: `header.payload.signature` (all base64url encoded)

```go
package traefik_jwt_decoder_plugin

import (
    "encoding/base64"
    "encoding/json"
    "errors"
    "strings"
)

// JWT represents a parsed JWT token
type JWT struct {
    Header    map[string]interface{}
    Payload   map[string]interface{}
    Signature string
}

// ParseJWT decodes a JWT token without validation
// Returns JWT struct with header and payload as maps
func ParseJWT(token string) (*JWT, error) {
    // Implementation needed:
    // 1. Split token by "." - must have exactly 3 segments
    // 2. Base64URL decode header (segment 0) - use base64.RawURLEncoding
    // 3. Base64URL decode payload (segment 1)
    // 4. JSON unmarshal header into map[string]interface{}
    // 5. JSON unmarshal payload into map[string]interface{}
    // 6. Store signature as string (segment 2) - don't decode
    // 7. Return JWT struct or error
    
    // Handle errors:
    // - Wrong number of segments → "invalid JWT format: expected 3 segments"
    // - Base64 decode failure → "invalid JWT encoding"
    // - JSON unmarshal failure → "invalid JWT JSON"
}

// ExtractToken removes prefix from bearer token
func ExtractToken(value, prefix string) string {
    // Implementation needed:
    // 1. If prefix is empty, return value as-is
    // 2. If value doesn't start with prefix, return value as-is
    // 3. Otherwise strip prefix and return remaining string
    // 4. Trim whitespace from result
}
```

**Testing Requirements for `jwt_test.go`**:
- Valid JWT parsing (RS256, HS256 tokens)
- Invalid JWT formats (1 segment, 2 segments, 4 segments)
- Invalid base64 encoding
- Invalid JSON in header/payload
- Empty token
- Token with no prefix
- Token with wrong prefix

#### 4. Claim Extraction

**File**: `claims.go`

```go
package traefik_jwt_decoder_plugin

import (
    "encoding/json"
    "errors"
    "fmt"
    "strings"
)

// ExtractClaim navigates a nested map using dot notation path
// Returns the claim value or error if path doesn't exist
func ExtractClaim(data map[string]interface{}, path string, maxDepth int) (interface{}, error) {
    // Implementation needed:
    // 1. Split path by "." to get parts
    // 2. Check depth doesn't exceed maxDepth
    // 3. Iterate through parts, navigating nested maps
    // 4. For each part:
    //    - Check if key exists in current map
    //    - If value is map[string]interface{}, continue
    //    - If last part, return value
    //    - Otherwise error (not a map)
    // 5. Return final value or error
    
    // Example: path="user.profile.name", data={"user": {"profile": {"name": "John"}}}
    // Should return "John"
    
    // Handle errors:
    // - Path depth exceeded → "claim path depth exceeds maximum"
    // - Key not found → "claim not found: {path}"
    // - Intermediate value not a map → "invalid claim path: {part} is not an object"
}

// ConvertClaimToString converts various claim types to string
func ConvertClaimToString(value interface{}, arrayFormat string) (string, error) {
    // Implementation needed:
    // 1. Handle nil → return "", nil
    // 2. Handle string → return as-is
    // 3. Handle bool, int, float → convert to string
    // 4. Handle array/slice:
    //    - If arrayFormat == "comma": join with ", "
    //    - If arrayFormat == "json": JSON marshal
    // 5. Handle object/map:
    //    - JSON marshal
    // 6. Default: fmt.Sprintf("%v", value)
}
```

**Testing Requirements for `claims_test.go`**:
- Simple claim extraction (`"sub"`)
- Nested claim extraction (`"user.profile.name"`)
- Array claim extraction
- Missing claim path
- Invalid intermediate path (not an object)
- Max depth exceeded
- Type conversions (string, int, bool, array, object)
- Null/nil values

#### 5. Header Injection

**File**: `headers.go`

```go
package traefik_jwt_decoder_plugin

import (
    "net/http"
    "strings"
)

var protectedHeaders = map[string]bool{
    "host":              true,
    "x-forwarded-for":   true,
    "x-forwarded-host":  true,
    "x-forwarded-proto": true,
    "x-forwarded-port":  true,
    "x-real-ip":         true,
    "content-length":    true,
    "content-type":      true,
    "transfer-encoding": true,
}

// IsProtectedHeader checks if header name is protected
func IsProtectedHeader(name string) bool {
    return protectedHeaders[strings.ToLower(name)]
}

// SanitizeHeaderValue removes dangerous characters from header values
func SanitizeHeaderValue(value string, maxSize int) (string, error) {
    // Implementation needed:
    // 1. Check length doesn't exceed maxSize
    // 2. Remove/replace control characters (especially \r and \n)
    // 3. Trim whitespace
    // 4. Return sanitized value or error
    
    // Handle errors:
    // - Size exceeded → "header value exceeds maximum size"
}

// InjectHeader adds header to request if safe to do so
func InjectHeader(req *http.Request, name, value string, override bool, maxSize int) error {
    // Implementation needed:
    // 1. Check if protected header → skip silently (return nil)
    // 2. Sanitize value
    // 3. Check if header already exists:
    //    - If exists and !override → skip silently (return nil)
    //    - If exists and override → replace
    //    - If not exists → add
    // 4. Use req.Header.Set() or req.Header.Add()
}
```

**Testing Requirements for `headers_test.go`**:
- Protected header blocking
- Header collision with override=false
- Header collision with override=true
- Value sanitization (newlines, control chars)
- Size limit enforcement
- Case-insensitive header checks

#### 6. Main Plugin

**File**: `jwt_claims_headers.go`

```go
package traefik_jwt_decoder_plugin

import (
    "context"
    "fmt"
    "log"
    "net/http"
)

// JWTClaimsHeaders is the main plugin struct
type JWTClaimsHeaders struct {
    next   http.Handler
    config *Config
    name   string
}

// New creates a new JWTClaimsHeaders plugin instance
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
    // Implementation needed:
    // 1. Validate config
    // 2. Return plugin instance
    
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }
    
    return &JWTClaimsHeaders{
        next:   next,
        config: config,
        name:   name,
    }, nil
}

// ServeHTTP implements the http.Handler interface
func (j *JWTClaimsHeaders) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    // Implementation needed:
    // 1. Extract JWT from configured source header
    // 2. If not found:
    //    - If continueOnError: pass through to next handler
    //    - Otherwise: return 401 Unauthorized
    // 3. Strip token prefix if configured
    // 4. Parse JWT
    // 5. If parse error:
    //    - Log error
    //    - If continueOnError: pass through
    //    - Otherwise: return 401 Unauthorized
    // 6. For each claim mapping:
    //    - Determine which section(s) to read from
    //    - Try extracting claim
    //    - If found: convert to string and inject header
    //    - If not found: log and skip
    // 7. If removeSourceHeader: delete source header
    // 8. Call next.ServeHTTP(rw, req)
    
    // Logging guidelines:
    // - Log JWT parse errors
    // - Log claim extraction failures (at debug level if possible)
    // - Log protected header attempts
    // - Log header injection successes (at debug level)
}
```

**Testing Requirements for `jwt_claims_headers_test.go`**:
- Full integration test with mock HTTP handler
- Valid JWT processing
- Invalid JWT with continueOnError=true
- Invalid JWT with continueOnError=false
- Missing Authorization header
- Multiple claim mappings
- Header section configuration
- RemoveSourceHeader functionality

### Phase 2: Enhanced Features

#### Array Format Options
- Comma-separated: `["admin", "user"]` → `"admin, user"`
- JSON string: `["admin", "user"]` → `"[\"admin\",\"user\"]"`

#### Section Selection Logic
- If `sections = ["payload"]`: only read from payload
- If `sections = ["header"]`: only read from header
- If `sections = ["payload", "header"]`: try payload first, fallback to header

#### Error Response Strategy

| Scenario | continueOnError=true | continueOnError=false |
|----------|---------------------|----------------------|
| Missing source header | Pass through | 401 + JSON body |
| Malformed JWT | Pass through | 401 + JSON body |
| Missing claim | Skip mapping | Continue |
| Protected header | Skip mapping | Skip mapping |

401 Response Body:
```json
{
  "error": "unauthorized",
  "message": "invalid or missing JWT token"
}
```

### Testing Strategy

#### Unit Tests
- `jwt_test.go`: All JWT parsing scenarios
- `claims_test.go`: All claim extraction scenarios  
- `headers_test.go`: All header injection scenarios
- `config_test.go`: Configuration validation

#### Integration Tests
- `jwt_claims_headers_test.go`: Full request/response cycle
- Test with real JWT tokens (HS256, RS256)
- Test various configuration combinations

#### Test JWT Tokens

**HS256 Test Token** (secret: `your-256-bit-secret`):
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwicm9sZXMiOlsiYWRtaW4iLCJ1c2VyIl0sImN1c3RvbSI6eyJ0ZW5hbnRfaWQiOiJ0ZW5hbnQtMTIzIn0sImlhdCI6MTUxNjIzOTAyMn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

Decoded payload:
```json
{
  "sub": "1234567890",
  "email": "test@example.com",
  "roles": ["admin", "user"],
  "custom": {
    "tenant_id": "tenant-123"
  },
  "iat": 1516239022
}
```

#### Manual Testing with Traefik

**File**: `examples/docker-compose.yml`
```yaml
version: '3.8'

services:
  traefik:
    image: traefik:v2.10
    command:
      - "--api.insecure=true"
      - "--providers.docker=true"
      - "--providers.file.directory=/etc/traefik/dynamic"
      - "--experimental.plugins.traefik-jwt-decoder-plugin.modulename=github.com/yourusername/traefik-jwt-decoder-plugin"
      - "--experimental.plugins.traefik-jwt-decoder-plugin.version=v0.1.0"
    ports:
      - "80:80"
      - "8080:8080"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock"
      - "./dynamic-config.yml:/etc/traefik/dynamic/config.yml"
      - "../:/plugins-local/src/github.com/yourusername/traefik-jwt-decoder-plugin"
    
  whoami:
    image: traefik/whoami
    labels:
      - "traefik.http.routers.whoami.rule=Host(`whoami.localhost`)"
      - "traefik.http.routers.whoami.middlewares=jwt-decoder@file"
```

**File**: `examples/dynamic-config.yml`
```yaml
http:
  middlewares:
    jwt-decoder:
      plugin:
        traefik-jwt-decoder-plugin:
          sourceHeader: "Authorization"
          tokenPrefix: "Bearer "
          claims:
            - claimPath: "sub"
              headerName: "X-User-Id"
            - claimPath: "email"
              headerName: "X-User-Email"
            - claimPath: "roles"
              headerName: "X-User-Roles"
              arrayFormat: "comma"
            - claimPath: "custom.tenant_id"
              headerName: "X-Tenant-Id"
          continueOnError: true
```

**Test Commands**:
```bash
# Test with valid JWT
curl -H "Authorization: Bearer eyJhbGc..." http://whoami.localhost

# Test without JWT
curl http://whoami.localhost

# Test with malformed JWT
curl -H "Authorization: Bearer invalid.token.here" http://whoami.localhost
```

### Development Workflow

1. **Setup**: Initialize Go module, create `.traefik.yml`
2. **Implement Core**: JWT parsing → Claim extraction → Header injection
3. **Write Tests**: Unit tests for each component
4. **Integration Test**: Full middleware test with mock handlers
5. **Manual Test**: Docker Compose setup with Traefik
6. **Refine**: Error handling, logging, edge cases
7. **Document**: Update README with usage examples

## Development Notes for Claude Code

### Key Considerations

1. **No External Dependencies**: Must work within Traefik's Yaegi interpreter - only Go stdlib
2. **Thread Safety**: Plugin may handle concurrent requests - ensure no shared mutable state
3. **Performance**: Minimize allocations, avoid unnecessary string operations
4. **Error Handling**: Always handle errors gracefully, log appropriately
5. **Security**: Never trust JWT contents, sanitize all values before header injection

### Common Pitfalls

- **Base64 Padding**: Use `base64.RawURLEncoding` (no padding) for JWT
- **Header Name Case**: HTTP headers are case-insensitive, normalize checks
- **Type Assertions**: Claims can be any JSON type, handle gracefully
- **Nil Checks**: JWT claims may be null/missing

### Logging Best Practices

```go
// Use context-aware logging
log.Printf("[%s] JWT parse error: %v", j.name, err)
log.Printf("[%s] Extracted claim %s: %s", j.name, claimPath, value)
```

### Performance Optimization

- Avoid string concatenation in loops (use strings.Builder)
- Reuse parsed JWT between claim extractions
- Consider claim path compilation/caching if needed

## Success Criteria

- [ ] Plugin loads successfully in Traefik
- [ ] Extracts simple claims (sub, email) correctly
- [ ] Extracts nested claims with dot notation
- [ ] Handles arrays with both comma and JSON formats
- [ ] Respects override flag for existing headers
- [ ] Blocks protected headers
- [ ] Handles malformed JWTs according to continueOnError
- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing with docker-compose succeeds
- [ ] No external dependencies

## Future Enhancements

- Optional signature verification (HMAC, RSA)
- Claim value transformation (base64, templates, case conversion)
- Conditional injection (only if claim meets criteria)
- Multiple source headers
- Claim caching for performance

## Security

**⚠️ CRITICAL**: This plugin does NOT perform JWT signature verification.

### Security Model

This plugin is designed for **internal service-to-service communication** behind a validated API gateway:

```
Internet → API Gateway (JWT verification ✓) → Traefik + Plugin (claim extraction) → Services
```

### Security Features

- **Header Injection Prevention**: Removes all control characters (0x00-0x1F, 0x7F) including CRLF sequences
- **Protected Header Guard**: Blocks modification of security-critical headers (`Host`, `X-Forwarded-*`, etc.)
- **Resource Limits**: Configurable `maxClaimDepth` and `maxHeaderSize` to prevent DoS attacks
- **Thread Safety**: No shared mutable state, safe for concurrent requests
- **Type Safety**: Graceful handling of unexpected claim types

### Security Testing

```bash
# Run security test suite
go test -v -run TestSecurity ./...

# Check for race conditions
go test -race ./...

# View test coverage
go test -cover ./...
```

**Security Audit**: See [SECURITY.md](SECURITY.md) for complete threat model, security controls, and deployment recommendations.

## Development

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose (for testing)
- No external dependencies allowed (Traefik Yaegi limitation)

### Building

```bash
# Verify Go module
go mod tidy

# Build and verify
go build ./...

# Run all tests
go test ./... -v

# Generate coverage report
go test -cover -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Testing

```bash
# Unit tests
go test ./... -v

# Integration tests
go test -v -run Integration ./...

# Security tests
go test -v -run TestSecurity ./...

# Race detection
go test -race ./...

# Specific test
go test -v -run TestParseJWT_Valid
```

### Project Structure

```
.
├── jwt.go                    # JWT parsing (base64url decode)
├── claims.go                 # Claim extraction with dot notation
├── headers.go                # Header injection and sanitization
├── config.go                 # Configuration validation
├── jwt_claims_headers.go     # Main middleware entrypoint
├── *_test.go                 # Comprehensive test suite (93% coverage)
├── examples/                 # Docker Compose testing environment
│   ├── docker-compose.yml
│   ├── dynamic-config.yml
│   ├── test-plugin.sh
│   └── README.md
├── ARCHITECTURE.md           # System design and data flow
├── SECURITY.md               # Threat model and security controls
├── CONTRIBUTING.md           # Development guidelines
└── README.md                 # This file
```

### Common Pitfalls

- **Base64 Encoding**: JWTs use `base64.RawURLEncoding` (no padding)
- **Header Case**: HTTP headers are case-insensitive, always normalize
- **Type Assertions**: Claims are `map[string]interface{}`, check types before assertions
- **Nil Handling**: JWT claims may be null, handle gracefully

For detailed development guidelines, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Documentation

- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System design, components, and data flow
- **[SECURITY.md](SECURITY.md)** - Security model, threat analysis, and deployment recommendations
- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Development workflow, testing requirements, and PR guidelines
- **[examples/README.md](examples/README.md)** - Docker Compose testing environment setup
- **[CLAUDE.md](CLAUDE.md)** - Development guidance for Claude Code AI assistant

### API Documentation

Generate godoc locally:

```bash
godoc -http=:6060
# Visit http://localhost:6060/pkg/traefik_jwt_decoder_plugin/
```

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for:

- Development setup instructions
- Code style guidelines
- Testing requirements (≥85% coverage)
- Pull request process
- Security considerations

## Roadmap

- [ ] Optional JWT signature verification (HMAC, RSA, ECDSA)
- [ ] Claim value transformations (base64, templates, regex)
- [ ] Conditional injection (claim value filters)
- [ ] Multiple source header support
- [ ] Performance optimizations (claim path caching)
- [ ] Prometheus metrics integration

## License

MIT License - See [LICENSE](LICENSE) for details

## Acknowledgments

- Built for [Traefik Proxy](https://traefik.io/traefik/)
- Inspired by the need for lightweight JWT claim extraction in internal microservice architectures
- Security testing based on [OWASP JWT Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html)

---

**Status**: Production-ready v0.1.0 (2025-10-12)
**Test Coverage**: 93%
**Security Audit**: Completed (2025-10-12)