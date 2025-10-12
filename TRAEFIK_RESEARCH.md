# Traefik Plugin System Architecture Research

**Research Date**: 2025-10-12
**Purpose**: Understanding Traefik middleware plugin implementation requirements for JWT claims extractor
**Target**: JWT decoder plugin (NO signature verification) for internal service-to-service communication

---

## 1. Architecture Overview (500 words)

### Plugin Loading and Initialization

Traefik plugins operate through a **two-stage initialization model**:

1. **Configuration Phase**: Traefik discovers plugins from public GitHub repositories (or local paths for development). Each plugin must have a `.traefik.yml` manifest at the repository root that declares metadata (display name, type, import path). The plugin's `go.mod` file must define the correct module path matching the import statement.

2. **Instantiation Phase**: When Traefik starts or reloads configuration, it calls the plugin's `New()` function for each middleware instance. This function receives:
   - `context.Context` - for cancellation and request-scoped values
   - `next http.Handler` - the next middleware or backend in the chain
   - `config *Config` - plugin-specific configuration from Traefik's dynamic config
   - `name string` - the middleware instance name

The `New()` function must return an `http.Handler` implementation or an error. If an error is returned, the plugin is disabled and Traefik logs the failure.

### Yaegi Interpreter: Why stdlib-only?

Traefik uses **Yaegi** (Yet Another Elegant Go Interpreter) to execute plugins "on the fly" without pre-compilation. This design choice provides several benefits:

1. **No compilation toolchain required** - plugins are deployed as source code
2. **Dynamic loading** - plugins can be added/updated without restarting Traefik
3. **Simplified distribution** - no binary compatibility concerns across platforms
4. **Security isolation** - interpreted code has limited access to system resources

However, Yaegi's **critical limitation** is that it only supports the **Go standard library**. This is because:

- Yaegi must pre-compile symbol tables for all available packages (`stdlib.Symbols`)
- Third-party packages would require dynamic symbol resolution, which is complex and error-prone
- External dependencies would introduce security risks and dependency hell
- The `unsafe` and `syscall` packages are explicitly excluded for security

**Impact for our plugin**: We can ONLY use packages from Go's standard library. For JWT parsing, this means using `encoding/base64`, `encoding/json`, and `strings` - we cannot use libraries like `github.com/golang-jwt/jwt`.

### ServeHTTP Contract and Middleware Chaining

Traefik middleware plugins implement the standard `http.Handler` interface:

```go
func (p *Plugin) ServeHTTP(rw http.ResponseWriter, req *http.Request)
```

**Critical rules for middleware chaining**:

1. **ALWAYS call `next.ServeHTTP(rw, req)`** - failure to do so breaks the middleware chain
2. **Call next handler AFTER processing** - middleware processes the request, then forwards
3. **Early termination** - if you don't want to forward the request (e.g., auth failure), write an error response and DON'T call next
4. **Thread safety** - ServeHTTP is called concurrently for multiple requests; avoid shared mutable state

**Execution order**: Traefik applies middlewares in the order they're declared in configuration. Request flows through middleware chain → backend service → response flows back through chain (reverse order for response processing).

### Configuration Handling Patterns

Plugins use a **struct-based configuration model**:

1. Define a `Config` struct with your plugin's settings
2. Use struct tags (typically `json:"fieldName,omitempty"`) for serialization
3. Implement `CreateConfig() *Config` to provide defaults
4. Validate configuration in the `New()` function, returning errors for invalid values

Traefik deserializes YAML/TOML configuration into your `Config` struct. Complex nested configurations are supported.

---

## 2. Key Implementation Findings

### Required Function Signatures

**Mandatory exports** (plugin WILL NOT WORK without these):

```go
// CreateConfig creates default plugin configuration
func CreateConfig() *Config {
    return &Config{
        // Initialize with sensible defaults
    }
}

// New instantiates the plugin
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
    // Validate config
    if err := validateConfig(config); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }

    // Return handler implementation
    return &MyPlugin{
        next:   next,
        config: config,
        name:   name,
    }, nil
}

// ServeHTTP implements http.Handler
func (p *MyPlugin) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    // Process request
    // ...

    // Forward to next handler (MANDATORY unless rejecting request)
    p.next.ServeHTTP(rw, req)
}
```

### Configuration Best Practices

From the official demo plugin and JWT plugin examples:

1. **Use maps for flexible key-value configs**: `map[string]string` for header mappings
2. **Provide defaults**: Always implement `CreateConfig()` with sensible defaults
3. **Validate early**: Check configuration in `New()`, not in `ServeHTTP()`
4. **Support optional fields**: Use `omitempty` in struct tags
5. **Nested structs for grouping**: Organize related settings into sub-structs

Example from our use case:
```go
type Config struct {
    SourceHeader      string                `json:"sourceHeader,omitempty"`
    Claims            []ClaimMapping        `json:"claims"`
    ContinueOnError   bool                  `json:"continueOnError,omitempty"`
    RemoveSourceHeader bool                 `json:"removeSourceHeader,omitempty"`
    MaxClaimDepth     int                   `json:"maxClaimDepth,omitempty"`
    MaxHeaderSize     int                   `json:"maxHeaderSize,omitempty"`
}

type ClaimMapping struct {
    ClaimPath  string   `json:"claimPath"`
    HeaderName string   `json:"headerName"`
    Sections   []string `json:"sections,omitempty"`
    Override   bool     `json:"override,omitempty"`
}
```

### Error Handling Patterns

**Two error handling strategies** observed in production plugins:

1. **Fail-closed (strict)**: Return HTTP 401/403 and DON'T call `next.ServeHTTP()`
   ```go
   if err := validateJWT(req); err != nil {
       http.Error(rw, "Unauthorized", http.StatusUnauthorized)
       return // Don't call next
   }
   ```

2. **Fail-open (permissive)**: Log error and continue to next handler
   ```go
   if err := extractClaims(req); err != nil {
       log.Printf("[%s] Error: %v", p.name, err)
       // Continue anyway
   }
   p.next.ServeHTTP(rw, req)
   ```

**Our plugin should support both** via `continueOnError` configuration flag.

**Error response format**: Use `http.Error()` for simple messages or `json.NewEncoder(rw).Encode()` for structured errors:
```go
rw.Header().Set("Content-Type", "application/json")
rw.WriteHeader(http.StatusUnauthorized)
json.NewEncoder(rw).Encode(map[string]string{
    "error": "invalid JWT",
    "detail": err.Error(),
})
```

### Logging Strategies

**Current Yaegi limitation**: Logging is primitive - only `os.Stdout.WriteString()` and `os.Stderr.WriteString()` are reliably available. The standard `log` package works but formatting is limited.

**Recommended pattern** from demo plugin:
```go
import "log"

// In ServeHTTP
log.Printf("[%s] Processing request: %s %s", p.name, req.Method, req.URL.Path)
```

**Best practices**:
- Include plugin `name` in all log messages for traceability
- Log errors to stderr, info to stdout
- Avoid excessive logging in hot path (every request)
- Log configuration validation errors in `New()`, not `ServeHTTP()`

---

## 3. Yaegi Limitations Affecting This Plugin

### Package Restrictions

**ALLOWED** (Go standard library only):
- `net/http` - HTTP handlers, requests, responses
- `encoding/base64` - JWT base64url decoding
- `encoding/json` - JWT payload parsing
- `strings` - String manipulation
- `context` - Context handling
- `fmt`, `errors` - Error handling
- `log` - Basic logging
- `os` - Environment, stdout/stderr
- `sync` - Mutexes, wait groups (if needed)

**FORBIDDEN** (will cause plugin to fail):
- Any `github.com/*` imports (e.g., `github.com/golang-jwt/jwt`)
- `crypto` package beyond basic hashing (signing/verification not in scope anyway)
- `syscall`, `unsafe` (security restriction)
- `reflect` (limited support, unreliable)
- Third-party libraries

### Reflection Limitations

**CRITICAL WARNING** from Yaegi documentation:

> "Representation of types by `reflect` and printing values using `%T` may give different results between compiled mode and interpreted mode"

**Impact**: Avoid `reflect` package entirely. Use type assertions carefully:

```go
// SAFE: Direct type assertion
if nested, ok := value.(map[string]interface{}); ok {
    // Use nested
}

// UNSAFE: Complex reflection operations
// DO NOT USE reflect.TypeOf, reflect.ValueOf in plugin
```

### Standard Library Exceptions

**Known working stdlib packages** (confirmed in Traefik plugins):
- All `encoding/*` packages (json, base64, etc.)
- `net/http` and `net/url`
- `strings`, `bytes`, `strconv`
- `time` (for timestamps)
- `fmt`, `errors`, `log`

**Potentially problematic**:
- `regexp` - works but may be slower in interpreted mode
- `crypto/*` - basic functionality works, but advanced features may fail
- `database/sql` - not tested, likely unsupported

### Known Workarounds

1. **No external JWT libraries**: Implement manual JWT parsing using `base64.RawURLEncoding` and `json.Unmarshal`

2. **Type assertions over reflection**: Use concrete type switches instead of reflect

3. **Avoid complex generic types**: Stick to `map[string]interface{}`, `[]interface{}`, basic types

4. **Test in Yaegi mode**: Differences between compiled and interpreted can be subtle

---

## 4. Code Examples from Similar Middleware

### Example 1: Header Injection Pattern (from plugindemo)

```go
func (a *Demo) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    // Iterate over configured headers
    for key, value := range a.headers {
        // Process value (template in this case, claims extraction in ours)
        processedValue := processValue(value, req)

        // Set header on request
        req.Header.Set(key, processedValue)
    }

    // Forward to next handler
    a.next.ServeHTTP(rw, req)
}
```

**Key insight**: Headers are modified on the `*http.Request` object BEFORE calling `next.ServeHTTP()`.

### Example 2: JWT Token Extraction (from traefik-jwt-plugin)

```go
// Extract Bearer token from Authorization header
func extractBearerToken(req *http.Request) (string, error) {
    authHeader := req.Header.Get("Authorization")
    if authHeader == "" {
        return "", errors.New("authorization header missing")
    }

    // Expected format: "Bearer <token>"
    parts := strings.Split(authHeader, " ")
    if len(parts) != 2 || parts[0] != "Bearer" {
        return "", errors.New("invalid authorization header format")
    }

    return parts[1], nil
}
```

**Key insight**: Use `req.Header.Get()` for case-insensitive header access.

### Example 3: Error Response with Conditional Logic

```go
func (p *Plugin) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    token, err := extractToken(req)
    if err != nil {
        if p.config.ContinueOnError {
            // Log and continue
            log.Printf("[%s] Warning: %v", p.name, err)
            p.next.ServeHTTP(rw, req)
            return
        }

        // Fail closed
        http.Error(rw, fmt.Sprintf("Unauthorized: %v", err), http.StatusUnauthorized)
        return // Don't call next
    }

    // Process token...
    p.next.ServeHTTP(rw, req)
}
```

**Key insight**: Early returns prevent calling `next.ServeHTTP()` when rejecting requests.

### Example 4: Protected Headers Check

```go
var protectedHeaders = []string{
    "host", "x-forwarded-for", "x-forwarded-proto", "x-real-ip",
    "content-length", "content-type", "transfer-encoding",
}

func isProtectedHeader(name string) bool {
    normalized := strings.ToLower(name)
    for _, protected := range protectedHeaders {
        if normalized == protected {
            return true
        }
    }
    return false
}

func setHeader(req *http.Request, name, value string) error {
    if isProtectedHeader(name) {
        return fmt.Errorf("cannot modify protected header: %s", name)
    }
    req.Header.Set(name, value)
    return nil
}
```

**Key insight**: Always normalize header names to lowercase for comparisons.

### Example 5: Configuration Validation in New()

```go
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
    // Validate required fields
    if len(config.Claims) == 0 {
        return nil, fmt.Errorf("claims array cannot be empty")
    }

    // Validate each claim mapping
    for i, claim := range config.Claims {
        if claim.ClaimPath == "" {
            return nil, fmt.Errorf("claim[%d]: claimPath is required", i)
        }
        if claim.HeaderName == "" {
            return nil, fmt.Errorf("claim[%d]: headerName is required", i)
        }
        if isProtectedHeader(claim.HeaderName) {
            return nil, fmt.Errorf("claim[%d]: cannot use protected header %s", i, claim.HeaderName)
        }
    }

    // Validate ranges
    if config.MaxClaimDepth <= 0 {
        return nil, fmt.Errorf("maxClaimDepth must be positive")
    }

    return &MyPlugin{next: next, config: config, name: name}, nil
}
```

**Key insight**: Thorough validation in `New()` prevents runtime errors in `ServeHTTP()`.

---

## 5. Implementation Recommendations for JWT Claims Decoder

Based on this research, here are the key implementation requirements:

### Plugin Structure
1. **Four core files**:
   - `jwt_claims_headers.go` - Main plugin with `New()` and `ServeHTTP()`
   - `config.go` - Configuration structs and validation
   - `jwt.go` - JWT parsing (base64, no verification)
   - `claims.go` - Claim extraction with dot notation
   - `headers.go` - Header injection with security checks

2. **Supporting files**:
   - `.traefik.yml` - Plugin manifest
   - `go.mod` - Module definition (no external dependencies)
   - `*_test.go` - Unit tests for each component

### Critical Implementation Details

1. **JWT Parsing**: Use `base64.RawURLEncoding` (no padding) to decode header and payload segments
2. **Claim Navigation**: Implement recursive descent for dot notation (`user.profile.email`)
3. **Type Handling**: Convert all claim values to strings (primitives, arrays, objects)
4. **Header Injection**: Use `req.Header.Set()` for setting, respect `override` flag
5. **Security Guards**: Protect critical headers, sanitize values (remove `\r`, `\n`)
6. **Error Handling**: Support both fail-open and fail-closed modes via config
7. **Thread Safety**: No shared mutable state; all data flows through request context

### Function Signatures

```go
// Main plugin
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)
func (j *JWTClaimsHeaders) ServeHTTP(rw http.ResponseWriter, req *http.Request)

// JWT operations
func parseJWT(token string) (*JWT, error)

// Claims operations
func extractClaim(claims map[string]interface{}, path string, maxDepth int) (string, error)

// Header operations
func injectHeaders(req *http.Request, mappings []HeaderMapping) error
func isProtectedHeader(name string) bool
func sanitizeHeaderValue(value string, maxSize int) (string, error)

// Config operations
func CreateConfig() *Config
func (c *Config) Validate() error
```

### Error Handling Strategy

- **Configuration errors**: Return from `New()` → plugin disabled
- **JWT parse errors**: Conditional based on `continueOnError`
- **Missing claims**: Log warning, skip header (don't fail request)
- **Header injection errors**: Log error, continue with other headers

### Testing Strategy

- **Unit tests**: Test each component independently (JWT parsing, claim extraction, header injection)
- **Integration tests**: Full request/response cycle with mock handlers
- **Yaegi compatibility tests**: Test actual interpreted execution, not just compiled Go

---

## 6. Conclusion

Traefik's plugin system is well-designed for extending middleware functionality, but the **Yaegi interpreter constraint** requires careful consideration:

✅ **Advantages**:
- No compilation required
- Simple deployment model
- Standard Go code (with stdlib restriction)
- Good documentation and examples

⚠️ **Challenges**:
- No external dependencies (must implement JWT parsing manually)
- Limited reflection support (avoid `reflect` package)
- Primitive logging (stdlib `log` package only)
- Performance slower than compiled middleware

For our JWT claims decoder plugin, the stdlib-only requirement is **manageable** because:
- We're NOT verifying signatures (no need for crypto libraries)
- JWT parsing is straightforward (base64 + JSON)
- Claim extraction is simple JSON navigation
- Header injection uses stdlib `net/http`

The research confirms that implementing this plugin with **only Go standard library is feasible and appropriate** for the use case.

---

## References

- Traefik Plugin Development Guide: https://plugins.traefik.io/create
- Official Plugin Demo: https://github.com/traefik/plugindemo
- Yaegi GitHub: https://github.com/traefik/yaegi
- JWT Plugin Example: https://github.com/traefik-plugins/traefik-jwt-plugin
- Traefik Middleware Documentation: https://doc.traefik.io/traefik/middlewares/http/overview/

---

**Research Complete**: Ready for implementation phase using this architecture guidance.
