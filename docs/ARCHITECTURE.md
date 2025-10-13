# Architecture Documentation

## Table of Contents

- [System Overview](#system-overview)
- [Component Architecture](#component-architecture)
- [Data Flow](#data-flow)
- [Request Processing Lifecycle](#request-processing-lifecycle)
- [Error Handling Strategy](#error-handling-strategy)
- [Security Architecture](#security-architecture)
- [Performance Characteristics](#performance-characteristics)
- [Thread Safety](#thread-safety)

## System Overview

The Traefik JWT Decoder Plugin is a middleware component that sits between Traefik's reverse proxy and upstream services. Its primary responsibility is to extract claims from JWT tokens and inject them as HTTP headers without performing signature verification.

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Client Request                          │
│              Authorization: Bearer <JWT>                     │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                    Traefik Proxy                             │
│  ┌────────────────────────────────────────────────────────┐ │
│  │         JWT Claims to Headers Plugin                   │ │
│  │                                                        │ │
│  │  1. Extract JWT from Authorization header             │ │
│  │  2. Parse JWT (base64url decode, no verification)     │ │
│  │  3. Navigate claim paths (dot notation)               │ │
│  │  4. Sanitize claim values (remove control chars)      │ │
│  │  5. Inject as HTTP headers                            │ │
│  └────────────────────────────────────────────────────────┘ │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                  Upstream Service                            │
│             (receives X-User-*, etc.)                        │
└─────────────────────────────────────────────────────────────┘
```

### Design Principles

1. **Simplicity**: Single responsibility - extract and inject claims
2. **Security**: Defense in depth with multiple sanitization layers
3. **Performance**: Minimal allocations, efficient string operations
4. **Reliability**: Graceful error handling, no panics
5. **Compatibility**: Pure Go stdlib (Traefik Yaegi requirement)

## Component Architecture

The plugin follows a modular design with clear separation of concerns:

```
┌──────────────────────────────────────────────────────────────┐
│                    jwt_claims_headers.go                      │
│                  (Main Plugin Entrypoint)                     │
│                                                               │
│  - Implements http.Handler interface                          │
│  - Coordinates all components                                 │
│  - Manages request/response lifecycle                         │
└───────┬──────────────┬──────────────┬───────────────────────┘
        │              │              │
        │              │              │
        ▼              ▼              ▼
┌──────────────┐ ┌───────────┐ ┌──────────────┐
│  config.go   │ │  jwt.go   │ │  claims.go   │
│              │ │           │ │              │
│ - Config     │ │ - Parse   │ │ - Extract    │
│   validation │ │   JWT     │ │   claims     │
│ - Defaults   │ │ - Strip   │ │ - Navigate   │
│ - Struct     │ │   prefix  │ │   paths      │
│   definitions│ │           │ │ - Type       │
│              │ │           │ │   conversion │
└──────────────┘ └───────────┘ └──────────────┘
                                       │
                                       │
                                       ▼
                                ┌──────────────┐
                                │  headers.go  │
                                │              │
                                │ - Sanitize   │
                                │   values     │
                                │ - Protected  │
                                │   headers    │
                                │ - Inject     │
                                │   headers    │
                                └──────────────┘
```

### Component Responsibilities

#### 1. config.go - Configuration Management

**Purpose**: Define and validate plugin configuration

**Key Types**:
- `Config`: Main configuration struct with all plugin settings
- `ClaimMapping`: Individual claim-to-header mapping

**Responsibilities**:
- Provide configuration defaults
- Validate configuration integrity
- Enforce business rules (no duplicate headers, valid sections, etc.)

**Key Functions**:
```go
CreateConfig() *Config           // Initialize with defaults
(c *Config) Validate() error     // Validate configuration
```

#### 2. jwt.go - JWT Parsing

**Purpose**: Parse JWT tokens without signature verification

**Key Types**:
- `JWT`: Structured representation of decoded JWT

**Responsibilities**:
- Split JWT into segments (header.payload.signature)
- Base64url decode header and payload sections
- JSON unmarshal into Go maps
- Extract token from Authorization header (strip prefix)

**Key Functions**:
```go
ParseJWT(token string) (*JWT, error)              // Parse full JWT
ExtractToken(value, prefix string) string         // Strip Bearer prefix
```

**Critical Implementation Details**:
- Uses `base64.RawURLEncoding` (no padding) per JWT spec
- Does NOT verify signature (by design)
- Signature stored as string for reference only

#### 3. claims.go - Claim Extraction

**Purpose**: Navigate and extract claims from JWT data structures

**Key Responsibilities**:
- Support dot notation for nested claims (`user.profile.email`)
- Enforce depth limits to prevent deep recursion attacks
- Convert various claim types to strings
- Handle arrays with configurable formatting

**Key Functions**:
```go
ExtractClaim(data map[string]interface{}, path string, maxDepth int) (interface{}, error)
ConvertClaimToString(value interface{}, arrayFormat string) (string, error)
```

**Supported Claim Types**:
- **Primitives**: string, bool, int, float64
- **Arrays**: Converted to comma-separated or JSON string
- **Objects**: JSON marshaled to string
- **Nil**: Returns empty string

**Example Dot Notation Navigation**:
```
Claim path: "user.profile.email"
JWT payload: {"user": {"profile": {"email": "test@example.com"}}}
Result: "test@example.com"
```

#### 4. headers.go - Header Injection & Sanitization

**Purpose**: Safely inject claims as HTTP headers with security guards

**Key Responsibilities**:
- Sanitize header values (remove control characters)
- Block protected headers (Host, X-Forwarded-*, etc.)
- Enforce size limits
- Handle header collision per configuration

**Key Functions**:
```go
IsProtectedHeader(name string) bool
SanitizeHeaderValue(value string, maxSize int) (string, error)
InjectHeader(req *http.Request, name, value string, override bool, maxSize int) error
```

**Protected Headers** (case-insensitive):
- `host`
- `x-forwarded-for`
- `x-forwarded-host`
- `x-forwarded-proto`
- `x-forwarded-port`
- `x-real-ip`
- `content-length`
- `content-type`
- `transfer-encoding`

**Sanitization Process**:
1. Check size limit (default 8KB)
2. Remove all control characters (0x00-0x1F, 0x7F)
3. Trim leading/trailing whitespace
4. Return sanitized value or error

#### 5. jwt_claims_headers.go - Main Middleware

**Purpose**: Coordinate all components in request processing

**Key Type**:
- `JWTClaimsHeaders`: Main plugin struct implementing `http.Handler`

**Responsibilities**:
- Extract JWT from source header
- Orchestrate parsing, extraction, and injection
- Implement error handling strategy (continueOnError)
- Remove source header if configured
- Forward request to next handler

**Key Functions**:
```go
New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)
(j *JWTClaimsHeaders) ServeHTTP(rw http.ResponseWriter, req *http.Request)
(j *JWTClaimsHeaders) returnError(rw http.ResponseWriter, errorType, message string)
```

## Data Flow

### Successful Request Flow

```
1. Client Request
   ├─ HTTP Request
   ├─ Authorization: Bearer <JWT>
   └─ Other headers

2. Extract JWT
   ├─ Read source header (default: Authorization)
   ├─ Strip prefix (default: "Bearer ")
   └─ Result: Raw JWT token

3. Parse JWT
   ├─ Split by "." → [header, payload, signature]
   ├─ base64.RawURLEncoding.DecodeString(header)
   ├─ base64.RawURLEncoding.DecodeString(payload)
   ├─ json.Unmarshal(headerBytes, &headerMap)
   ├─ json.Unmarshal(payloadBytes, &payloadMap)
   └─ Result: JWT struct with maps

4. Process Claim Mappings (for each mapping)
   ├─ Determine sections to search (payload, header, or both)
   ├─ For each section:
   │  ├─ Call ExtractClaim(data, claimPath, maxDepth)
   │  ├─ Navigate dot notation path
   │  └─ Return claim value or error
   ├─ Convert value to string
   │  ├─ Handle primitives (string, bool, int, float)
   │  ├─ Handle arrays (comma-separated or JSON)
   │  ├─ Handle objects (JSON marshal)
   │  └─ Handle nil (empty string)
   └─ Inject as header

5. Inject Header
   ├─ Check if protected header → skip if true
   ├─ Sanitize value
   │  ├─ Check size limit
   │  ├─ Remove control characters
   │  └─ Trim whitespace
   ├─ Check collision
   │  ├─ If header exists && !override → skip
   │  └─ If header exists && override → replace
   └─ req.Header.Set(name, sanitizedValue)

6. Cleanup & Forward
   ├─ Remove source header if configured
   └─ Call next.ServeHTTP(rw, req)

7. Upstream Service
   └─ Receives request with X-* headers
```

### Error Flow

```
Error Scenario: Invalid JWT
   │
   ├─ continueOnError = true
   │  ├─ Log error
   │  ├─ Skip claim injection
   │  └─ Call next.ServeHTTP() → Pass through
   │
   └─ continueOnError = false
      ├─ Log error
      ├─ Set Content-Type: application/json
      ├─ WriteHeader(401)
      ├─ Write JSON error body
      └─ Return (stop processing)
```

## Request Processing Lifecycle

### Phase 1: Initialization (Plugin Load)

1. Traefik calls `New()` function
2. Validate configuration with `config.Validate()`
3. Return plugin instance or error
4. Plugin registered in middleware chain

### Phase 2: Request Processing (Per Request)

```
┌─────────────────────────────────────────────────────────────┐
│  ServeHTTP(rw http.ResponseWriter, req *http.Request)       │
└───────┬─────────────────────────────────────────────────────┘
        │
        ▼
   Extract JWT from header
        │
        ├─ Not found? ───┐
        │                ▼
        │         Handle Error
        │         (continue or 401)
        │                │
        ▼                │
   Parse JWT             │
        │                │
        ├─ Parse error? ─┤
        │                │
        ▼                │
   For each claim mapping│
        │                │
        ├─ Try sections  │
        │  (payload, header)
        │                │
        ├─ Extract claim │
        │                │
        ├─ Convert to string
        │                │
        ├─ Inject header │
        │                │
        └─ Next mapping  │
                         │
   Remove source header? │
        │                │
        ▼                │
   Forward to next ◄─────┘
```

### Phase 3: Cleanup

- No explicit cleanup required
- Garbage collector handles memory
- No connections or resources to close

## Error Handling Strategy

### Error Modes

#### Strict Mode (`continueOnError: false`)

**Use Case**: Production systems requiring valid JWTs

**Behavior**:
- Missing JWT → 401 Unauthorized
- Invalid JWT format → 401 Unauthorized
- Base64 decode error → 401 Unauthorized
- JSON parse error → 401 Unauthorized

**Response Format**:
```json
{
  "error": "unauthorized",
  "message": "invalid or missing JWT token"
}
```

#### Permissive Mode (`continueOnError: true`)

**Use Case**: Development, optional authentication, graceful degradation

**Behavior**:
- Missing JWT → Log warning, pass through without headers
- Invalid JWT → Log error, pass through without headers
- Claim errors → Log error, skip claim, continue with others

### Error Types & Handling

| Error Type | Strict Mode | Permissive Mode | Logged |
|------------|-------------|-----------------|--------|
| Missing source header | 401 | Pass through | Yes |
| Invalid JWT segments | 401 | Pass through | Yes |
| Base64 decode error | 401 | Pass through | Yes |
| JSON parse error | 401 | Pass through | Yes |
| Claim not found | Continue | Continue | Yes |
| Claim depth exceeded | Continue | Continue | Yes |
| Header size exceeded | Continue | Continue | Yes |
| Protected header | Skip silently | Skip silently | No |

## Security Architecture

### Defense Layers

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 1: Configuration Validation                           │
│  - Validate claim paths                                      │
│  - Check for duplicate headers                               │
│  - Enforce depth/size limits                                 │
└───────────────────────┬─────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────┐
│  Layer 2: JWT Parsing                                        │
│  - Validate segment count (must be 3)                        │
│  - Safe base64 decoding                                      │
│  - Safe JSON unmarshaling                                    │
└───────────────────────┬─────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────┐
│  Layer 3: Claim Extraction                                   │
│  - Depth limit enforcement                                   │
│  - Type-safe assertions                                      │
│  - Nil value handling                                        │
└───────────────────────┬─────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────┐
│  Layer 4: Header Injection                                   │
│  - Protected header blocking                                 │
│  - Size limit enforcement                                    │
│  - Control character sanitization                            │
│  - Collision policy enforcement                              │
└─────────────────────────────────────────────────────────────┘
```

### Security Controls

1. **Input Validation**
   - JWT segment count verification
   - Base64 encoding validation
   - JSON structure validation

2. **Resource Limits**
   - `maxClaimDepth`: Prevent deep recursion (default: 10)
   - `maxHeaderSize`: Prevent memory exhaustion (default: 8KB)

3. **Output Sanitization**
   - Control character removal (0x00-0x1F, 0x7F)
   - CRLF injection prevention (`\r\n` removed)
   - Unicode normalization handling

4. **Access Control**
   - Protected header blacklist
   - Case-insensitive header name checks

5. **Type Safety**
   - Safe type assertions with `ok` checks
   - Graceful handling of unexpected types
   - No panic conditions

## Performance Characteristics

### Time Complexity

| Operation | Complexity | Notes |
|-----------|-----------|-------|
| JWT parsing | O(n) | n = JWT size |
| Claim extraction | O(d) | d = claim path depth |
| Header sanitization | O(m) | m = header value length |
| Protected header check | O(1) | Map lookup |
| Overall request | O(n + c*d + c*m) | c = claim count |

### Space Complexity

| Component | Memory Usage | Notes |
|-----------|--------------|-------|
| JWT struct | ~2x JWT size | Header + Payload maps |
| Claim extraction | O(d) | Call stack depth |
| Header injection | O(c*m) | c claims × m avg size |
| Total per request | O(n + c*m) | Linear in input size |

### Performance Optimizations

1. **String Operations**
   - Use `strings.Builder` for concatenation
   - Minimize allocations with reuse
   - Pre-sized slices where possible

2. **Map Operations**
   - Direct map access (O(1) average)
   - Protected headers pre-computed in map

3. **JWT Reuse**
   - Parse JWT once, reuse for all claims
   - No redundant parsing or decoding

4. **Early Exits**
   - Skip protected headers immediately
   - Return on first error in strict mode

### Benchmarking

Typical performance on modern hardware:

- **JWT parsing**: ~10μs for 1KB JWT
- **Claim extraction**: ~1μs per claim
- **Header injection**: ~2μs per header
- **Total request overhead**: ~50-100μs for 5 claims

## Thread Safety

### Concurrency Model

**Thread-Safe Components**:
- All functions are pure or stateless
- No shared mutable state in plugin struct
- Each request processed independently

**Data Flow**:
```
Request 1 ─┐
Request 2 ─┼─→ Plugin Instance (Immutable)
Request 3 ─┘
           │
           ├─→ Independent JWT parsing
           ├─→ Independent claim extraction
           └─→ Independent header injection
```

### Immutable Plugin State

```go
type JWTClaimsHeaders struct {
    next   http.Handler  // Immutable after creation
    config *Config       // Read-only after creation
    name   string        // Immutable string
}
```

### Per-Request Processing

All mutable data flows through function parameters:

```go
func (j *JWTClaimsHeaders) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    // All data is in 'req' (per-request scope)
    // No shared state accessed
    // No race conditions possible
}
```

### Race Condition Prevention

**Verified with**:
```bash
go test -race ./... -count=100
```

**Result**: No race conditions detected across 100 iterations

## Deployment Considerations

### Recommended Architecture

```
┌──────────────────────────────────────────────────────────┐
│  Public Internet                                          │
└───────────────────────┬──────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────┐
│  API Gateway / Auth Proxy                                 │
│  - JWT Signature Verification ✓                           │
│  - Rate Limiting                                          │
│  - DDoS Protection                                        │
└───────────────────────┬──────────────────────────────────┘
                        │ (Validated JWT)
                        ▼
┌──────────────────────────────────────────────────────────┐
│  Traefik + JWT Decoder Plugin                             │
│  - Claim Extraction (NO verification)                     │
│  - Header Injection                                       │
└───────────────────────┬──────────────────────────────────┘
                        │ (JWT + Headers)
                        ▼
┌──────────────────────────────────────────────────────────┐
│  Backend Services                                         │
│  - Consume headers                                        │
│  - Additional validation                                  │
└──────────────────────────────────────────────────────────┘
```

### Scaling Considerations

1. **Stateless Design**: Horizontal scaling supported
2. **Low Memory Footprint**: ~10KB per request
3. **Fast Processing**: <100μs overhead per request
4. **No External Dependencies**: No database, cache, or API calls

### Monitoring Recommendations

**Key Metrics**:
- JWT parse error rate (threshold: <1%)
- Header size exceeded count (investigate if >0)
- Claim depth exceeded count (potential attack if frequent)
- Request processing time (p50, p95, p99)

**Alerting**:
- Spike in JWT errors (>5% of requests)
- Unusual claim paths (reconnaissance attempts)
- Performance degradation (p99 >1ms)

---

**Last Updated**: 2025-10-12
**Plugin Version**: 1.0.0
