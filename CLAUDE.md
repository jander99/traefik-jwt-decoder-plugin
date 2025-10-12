# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Traefik middleware plugin that extracts claims from JWT tokens and injects them as HTTP headers. The plugin decodes JWTs **without signature verification** - it's designed for internal service-to-service communication where JWT validation happens at the edge.

**Critical Constraint**: This plugin must work within Traefik's Yaegi interpreter, which means **NO external dependencies** - only Go standard library is allowed.

## Architecture

The codebase follows a modular design with clear separation of concerns:

```
jwt_claims_headers.go   # Main plugin entrypoint (ServeHTTP middleware)
config.go              # Configuration structs & validation
jwt.go                 # JWT parsing (base64 decode, no verification)
claims.go              # Claim extraction with dot notation support
headers.go             # Header injection with security guards
```

**Data Flow**:
1. Extract JWT from HTTP header (default: `Authorization`)
2. Parse JWT (base64url decode header and payload sections)
3. For each configured claim mapping:
   - Navigate claim path using dot notation (e.g., `user.profile.name`)
   - Extract value from JWT header or payload section
   - Convert to string (handle arrays, objects, primitives)
   - Inject as HTTP header (with collision and protection checks)
4. Forward to upstream service

## Development Commands

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -cover ./...

# Run specific test file
go test -v ./jwt_test.go

# Run tests with race detection
go test -race ./...
```

### Building
```bash
# Verify Go module
go mod tidy

# Verify code compiles
go build ./...

# Run linter (if golangci-lint installed)
golangci-lint run
```

### Manual Testing with Traefik
```bash
# Start Traefik with plugin (from examples/ directory)
docker-compose up

# Test with JWT token
curl -H "Authorization: Bearer <JWT>" http://whoami.localhost

# View injected headers
curl -H "Authorization: Bearer <JWT>" http://whoami.localhost | grep X-
```

## Implementation Guidelines

### JWT Parsing (jwt.go)

**Base64 Encoding**: Use `base64.RawURLEncoding` (no padding) for JWT segments.

**Structure**: JWT format is `header.payload.signature` where:
- Header and payload are base64url-encoded JSON objects
- Signature is base64url-encoded but not decoded (stored as string)

**Error Handling**:
- Wrong segment count (must be exactly 3)
- Invalid base64 encoding
- Invalid JSON structure

### Claim Extraction (claims.go)

**Dot Notation**: Support nested claim paths like `user.profile.email`.

**Type Handling**:
- Primitives (string, bool, int, float): Direct conversion
- Arrays: Support both comma-separated (`"admin, user"`) and JSON format (`["admin","user"]`)
- Objects: JSON marshal to string
- Null/nil: Return empty string

**Depth Limits**: Enforce `maxClaimDepth` to prevent deep recursion attacks.

### Header Injection (headers.go)

**Protected Headers**: Never inject into these headers (case-insensitive check):
- `Host`, `X-Forwarded-*`, `X-Real-IP`
- `Content-Length`, `Content-Type`, `Transfer-Encoding`

**Sanitization**:
- Remove control characters (especially `\r` and `\n` for header injection attacks)
- Enforce `maxHeaderSize` limit
- Trim whitespace

**Collision Handling**:
- If header exists and `override=false`: skip silently
- If header exists and `override=true`: replace with `req.Header.Set()`
- If header doesn't exist: add with `req.Header.Set()`

### Configuration (config.go)

**Validation Requirements**:
- Claims array must not be empty
- Each claim mapping requires `claimPath` and `headerName`
- Sections must only contain `"header"` or `"payload"`
- `maxClaimDepth` and `maxHeaderSize` must be positive
- No duplicate `headerName` values

**Section Selection Logic**:
- `["payload"]`: Only read from payload
- `["header"]`: Only read from JWT header
- `["payload", "header"]`: Try payload first, fallback to header
- Multiple sections: Try in order until claim found

### Main Plugin (jwt_claims_headers.go)

**Error Strategy**:
- If `continueOnError=true`: Log errors and pass request through
- If `continueOnError=false`: Return 401 with JSON error body

**Thread Safety**: No shared mutable state - all data flows through request context.

**Logging**: Use contextual logging with plugin name:
```go
log.Printf("[%s] JWT parse error: %v", j.name, err)
```

## Testing Strategy

### Unit Tests
- `jwt_test.go`: Valid/invalid JWT formats, base64 errors, JSON errors
- `claims_test.go`: Simple/nested paths, arrays, missing claims, depth limits
- `headers_test.go`: Protected headers, collisions, sanitization, size limits
- `config_test.go`: Validation rules for all config fields

### Integration Tests
- `jwt_claims_headers_test.go`: Full request/response cycle with mock handlers
- Test with real JWT tokens (HS256 example in README)
- Test `continueOnError` behavior
- Test `removeSourceHeader` functionality

### Test JWT Token
Use this HS256 token for testing (secret: `your-256-bit-secret`):
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwicm9sZXMiOlsiYWRtaW4iLCJ1c2VyIl0sImN1c3RvbSI6eyJ0ZW5hbnRfaWQiOiJ0ZW5hbnQtMTIzIn0sImlhdCI6MTUxNjIzOTAyMn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

Payload contains: `sub`, `email`, `roles` (array), `custom.tenant_id` (nested).

## Common Pitfalls

1. **Base64 Padding**: JWTs use base64url encoding **without padding**. Always use `base64.RawURLEncoding`.

2. **Header Case Sensitivity**: HTTP headers are case-insensitive. Always use `strings.ToLower()` for comparisons.

3. **Type Assertions**: JWT claims are `map[string]interface{}`. Check types before assertions:
   ```go
   if nested, ok := value.(map[string]interface{}); ok {
       // Safe to use as map
   }
   ```

4. **Nil Checks**: Claims may be null in JWT. Handle gracefully:
   ```go
   if value == nil {
       return "", nil
   }
   ```

5. **Concurrent Requests**: Plugin must be thread-safe. Avoid shared mutable state.

## Performance Considerations

- Avoid string concatenation in loops (use `strings.Builder`)
- Parse JWT once and reuse for all claim extractions
- Minimize memory allocations in hot path
- Consider pre-compiling claim paths if optimization needed

## Security Notes

1. **No Signature Verification**: This plugin does NOT verify JWT signatures. It's designed for internal networks where verification happens at the edge.

2. **Input Sanitization**: All claim values must be sanitized before header injection to prevent header injection attacks.

3. **Protected Headers**: Never allow overriding security-critical headers like `X-Forwarded-For`.

4. **Size Limits**: Enforce `maxHeaderSize` to prevent memory exhaustion.

5. **Depth Limits**: Enforce `maxClaimDepth` to prevent deep recursion.

## File Creation Order

If implementing from scratch, create files in this order:
1. `go.mod` and `.traefik.yml` (project setup)
2. `config.go` (data structures)
3. `jwt.go` (parsing logic)
4. `claims.go` (extraction logic)
5. `headers.go` (injection logic)
6. `jwt_claims_headers.go` (main plugin)
7. Test files for each component

## Traefik Plugin Requirements

**Module Structure**: Must have `go.mod` with module path matching plugin import.

**Required File**: `.traefik.yml` defines plugin metadata:
```yaml
displayName: JWT Claims to Headers
type: middleware
import: github.com/yourusername/traefik-jwt-decoder-plugin
```

**Plugin Function**: Must export `New` function with signature:
```go
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)
```

**Dependencies**: Only Go standard library allowed (Yaegi interpreter limitation).
