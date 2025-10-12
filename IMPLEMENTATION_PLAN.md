# Traefik JWT Decoder Plugin - Implementation Plan

**Status**: Ready for parallel agent execution
**Generated**: 2025-10-12
**Project**: traefik-jwt-decoder-plugin

## Executive Summary

This implementation plan coordinates multiple specialized agents to develop, test, and document a Traefik middleware plugin that extracts JWT claims and injects them as HTTP headers. All agents MUST use context7 and deepwiki MCP servers to research Traefik plugin architecture before writing code.

## Current State Analysis

**Existing Structure**:
- ✅ Stub implementations created (jwt.go, claims.go, headers.go, config.go, jwt_claims_headers.go)
- ✅ Test file stubs created (*_test.go)
- ✅ Comprehensive README with specification
- ✅ CLAUDE.md with development guidance
- ⚠️ All implementations marked with TODO
- ⚠️ No git commits yet (files untracked)
- ⚠️ Examples directory exists but needs validation

**Critical Constraints**:
- NO external dependencies (Traefik Yaegi interpreter limitation)
- Only Go standard library allowed
- Thread-safe implementation required
- NO JWT signature verification (by design)

## Phase 0: Research & Architecture Understanding

**Agent**: `ai-researcher` or `go-engineer`
**Priority**: CRITICAL - Must complete before Phase 1
**Traits**: `enhancement/mcp-integration`, `safety/context-verification`

### Task 0.1: Traefik Plugin System Research

**MANDATORY**: Use MCP servers before any code implementation:

```bash
# Using context7 to fetch Traefik plugin documentation
mcp__context7__resolve-library-id(libraryName: "traefik")
mcp__context7__get-library-docs(context7CompatibleLibraryID: "/traefik/traefik", topic: "plugins middleware")

# Using deepwiki for additional context
mcp__deepwiki__deepwiki_fetch(url: "traefik/traefik", maxDepth: 1, mode: "aggregate")
```

**Research Objectives**:
1. Understand Traefik plugin architecture and lifecycle
2. Identify Yaegi interpreter limitations and workarounds
3. Study middleware chain execution order
4. Review existing JWT-related middleware patterns
5. Understand ServeHTTP contract and request/response handling

**Deliverables**:
- Architecture summary document (300-500 words)
- List of Yaegi limitations affecting this plugin
- Best practices for Traefik middleware development
- Examples of similar middleware implementations

**Success Criteria**:
- Agent demonstrates understanding of Traefik plugin loading mechanism
- Can explain why no external dependencies are allowed
- Understands http.Handler interface requirements

## Phase 1: Core Implementation (Parallel Execution)

**Coordination Pattern**: Multiple agents work in parallel with isolated contexts

### Stream 1A: JWT Parsing Logic

**Agent**: `go-engineer`
**File**: `jwt.go`
**Traits**: `safety/branch-check`, `coordination/testing-handoff`

**Prerequisites**:
- Phase 0 research completed
- Understanding of base64url encoding (no padding)
- Knowledge of JWT structure: `header.payload.signature`

**Implementation Requirements**:

```go
// ParseJWT implementation checklist:
// 1. Split token by "." → must have exactly 3 segments
// 2. Decode segment 0 (header) using base64.RawURLEncoding
// 3. Decode segment 1 (payload) using base64.RawURLEncoding
// 4. Unmarshal both into map[string]interface{}
// 5. Store segment 2 (signature) as string (no decode)
// 6. Return JWT struct or descriptive error

// ExtractToken implementation checklist:
// 1. Handle empty prefix case
// 2. Check if value starts with prefix
// 3. Strip prefix using strings.TrimPrefix()
// 4. Trim whitespace with strings.TrimSpace()
```

**Error Handling**:
- Wrong segment count: `"invalid JWT format: expected 3 segments, got %d"`
- Base64 decode error: `"invalid JWT encoding: %v"`
- JSON unmarshal error: `"invalid JWT JSON: %v"`

**Handoff**: Coordinate with `qa-engineer` via `coordination/testing-handoff` for jwt_test.go

### Stream 1B: Configuration Validation

**Agent**: `go-engineer`
**File**: `config.go`
**Traits**: `safety/branch-check`, `coordination/testing-handoff`

**Implementation Requirements**:

```go
// Validate() implementation checklist:
// 1. Check Claims array not empty
// 2. For each ClaimMapping:
//    - Validate ClaimPath not empty
//    - Validate HeaderName not empty
//    - Check ArrayFormat is "comma", "json", or empty
// 3. Validate Sections contains only "header" or "payload"
// 4. Check MaxClaimDepth > 0
// 5. Check MaxHeaderSize > 0
// 6. Detect duplicate HeaderName values
// 7. Return descriptive errors for each violation
```

**Validation Rules**:
- Claims must have at least one mapping
- No duplicate header names (case-insensitive check)
- Sections array must not be empty
- Both maxClaimDepth and maxHeaderSize must be positive

**Handoff**: Coordinate with `qa-engineer` for config_test.go

### Stream 1C: Claim Extraction Logic

**Agent**: `go-engineer`
**File**: `claims.go`
**Traits**: `safety/branch-check`, `coordination/testing-handoff`

**Implementation Requirements**:

```go
// ExtractClaim implementation checklist:
// 1. Split path by "." to get parts array
// 2. Validate len(parts) <= maxDepth
// 3. Start with current = data (map[string]interface{})
// 4. For each part in parts:
//    a. Check if key exists in current map
//    b. Get value from current[part]
//    c. If last part: return value
//    d. Otherwise: type assert to map[string]interface{}
//    e. If assertion fails: error "not an object"
//    f. Set current = nested map
// 5. Return final value or error

// ConvertClaimToString implementation checklist:
// 1. nil → return "", nil
// 2. string → return as-is
// 3. bool → strconv.FormatBool()
// 4. float64 → strconv.FormatFloat()
// 5. int/int64 → strconv.FormatInt()
// 6. []interface{} (array):
//    - arrayFormat="comma": join with ", "
//    - arrayFormat="json": json.Marshal()
// 7. map[string]interface{}: json.Marshal()
// 8. Default: fmt.Sprintf("%v", value)
```

**Special Cases**:
- Nested arrays: `roles[0]` not supported (document limitation)
- Empty string claims: valid, return ""
- Numeric claims: convert to string without scientific notation

**Handoff**: Coordinate with `qa-engineer` for claims_test.go

### Stream 1D: Header Injection Security

**Agent**: `security-engineer`
**File**: `headers.go`
**Traits**: `safety/branch-check`, `coordination/testing-handoff`

**Security Requirements**:

```go
// Protected headers (case-insensitive blocking):
var protectedHeaders = map[string]bool{
    "host": true,
    "x-forwarded-for": true,
    "x-forwarded-host": true,
    "x-forwarded-proto": true,
    "x-forwarded-port": true,
    "x-real-ip": true,
    "content-length": true,
    "content-type": true,
    "transfer-encoding": true,
}

// SanitizeHeaderValue implementation checklist:
// 1. Check len(value) <= maxSize
// 2. Remove all control characters (0x00-0x1F, 0x7F)
// 3. Specifically remove \r and \n (header injection attack)
// 4. Trim leading/trailing whitespace
// 5. Return sanitized string or error

// InjectHeader implementation checklist:
// 1. Normalize header name to lowercase for protected check
// 2. If IsProtectedHeader(): log warning and return nil (skip)
// 3. Call SanitizeHeaderValue()
// 4. Check req.Header.Get(name) != ""
//    - If exists && !override: return nil (skip)
//    - If exists && override: req.Header.Set(name, value)
//    - If not exists: req.Header.Set(name, value)
```

**Security Tests Required**:
- Header injection attack: `"value\r\nX-Evil: injected"`
- Control character removal: `"value\x00\x01\x1F"`
- Protected header blocking: attempt to set `Host`, `X-Forwarded-For`
- Case insensitivity: `X-FORWARDED-FOR`, `x-forwarded-for`
- Size limit enforcement: 8KB default, configurable

**Handoff**: Coordinate with `qa-engineer` for headers_test.go + security test suite

## Phase 2: Main Middleware Integration

**Agent**: `go-engineer`
**File**: `jwt_claims_headers.go`
**Dependencies**: Streams 1A, 1B, 1C, 1D completed
**Traits**: `safety/branch-check`, `coordination/testing-handoff`

### Task 2.1: ServeHTTP Implementation

**Execution Flow**:

```go
// ServeHTTP implementation checklist:
// 1. Extract JWT from req.Header.Get(config.SourceHeader)
// 2. If empty:
//    - Log: "JWT source header not found"
//    - If continueOnError: call next.ServeHTTP()
//    - Else: return 401 with JSON error body
// 3. Strip prefix: token = ExtractToken(headerValue, config.TokenPrefix)
// 4. Parse JWT: jwt, err = ParseJWT(token)
// 5. If parse error:
//    - Log: "JWT parse error: %v"
//    - If continueOnError: call next.ServeHTTP()
//    - Else: return 401 with JSON error body
// 6. For each claim in config.Claims:
//    a. Determine sections to search (payload, header, or both)
//    b. Try extracting from each section in order
//    c. If found: ConvertClaimToString()
//    d. InjectHeader(req, claim.HeaderName, strValue, claim.Override, maxHeaderSize)
//    e. If not found: log at debug level
// 7. If config.RemoveSourceHeader: req.Header.Del(config.SourceHeader)
// 8. Call next.ServeHTTP(rw, req)
```

**Error Response Format** (continueOnError=false):
```json
{
  "error": "unauthorized",
  "message": "invalid or missing JWT token"
}
```

**Section Search Logic**:
- `["payload"]`: Only jwt.Payload
- `["header"]`: Only jwt.Header
- `["payload", "header"]`: Try Payload first, fallback to Header
- `["header", "payload"]`: Try Header first, fallback to Payload

**Logging Strategy**:
```go
log.Printf("[%s] JWT source header not found: %s", j.name, config.SourceHeader)
log.Printf("[%s] JWT parse error: %v", j.name, err)
log.Printf("[%s] Claim not found: %s", j.name, claimPath)
log.Printf("[%s] Injected header: %s", j.name, headerName)
```

## Phase 3: Comprehensive Testing

**Coordination**: `qa-engineer` coordinates all testing activities
**Traits**: `coordination/testing-handoff` (receives from all go-engineer streams)

### Task 3.1: Unit Test Implementation

**Test Files Priority Order**:
1. `jwt_test.go` (dependencies: none)
2. `config_test.go` (dependencies: none)
3. `claims_test.go` (dependencies: jwt.go)
4. `headers_test.go` (dependencies: none)
5. `jwt_claims_headers_test.go` (dependencies: all)

#### jwt_test.go Test Cases

```go
// Test cases checklist:
// 1. TestParseJWT_Valid
//    - HS256 token from README
//    - RS256 token
//    - Token with special characters in claims
// 2. TestParseJWT_InvalidSegmentCount
//    - 1 segment: "token"
//    - 2 segments: "header.payload"
//    - 4 segments: "a.b.c.d"
// 3. TestParseJWT_InvalidBase64
//    - Invalid characters in header
//    - Invalid characters in payload
// 4. TestParseJWT_InvalidJSON
//    - Non-JSON header
//    - Non-JSON payload
// 5. TestExtractToken_WithPrefix
//    - "Bearer token123" → "token123"
//    - "Bearer  token123" → "token123" (extra space)
// 6. TestExtractToken_WithoutPrefix
//    - "token123" → "token123"
// 7. TestExtractToken_EmptyPrefix
//    - Empty prefix config → return as-is
```

#### claims_test.go Test Cases

```go
// Test cases checklist:
// 1. TestExtractClaim_Simple
//    - "sub" → "1234567890"
//    - "email" → "test@example.com"
// 2. TestExtractClaim_Nested
//    - "custom.tenant_id" → "tenant-123"
//    - "user.profile.name" → "John Doe"
// 3. TestExtractClaim_NotFound
//    - "nonexistent" → error
//    - "user.missing.path" → error
// 4. TestExtractClaim_DepthExceeded
//    - Path with 11 levels (maxDepth=10)
// 5. TestExtractClaim_InvalidPath
//    - "roles.name" where roles is array
// 6. TestConvertClaimToString_Primitives
//    - string, int, float, bool
// 7. TestConvertClaimToString_Array
//    - ["admin", "user"] with arrayFormat="comma"
//    - ["admin", "user"] with arrayFormat="json"
// 8. TestConvertClaimToString_Object
//    - {"key": "value"} → JSON string
// 9. TestConvertClaimToString_Nil
//    - nil → ""
```

#### headers_test.go Test Cases

```go
// Test cases checklist:
// 1. TestIsProtectedHeader
//    - All protected headers (various cases)
// 2. TestSanitizeHeaderValue_Valid
//    - Normal string
// 3. TestSanitizeHeaderValue_ControlChars
//    - "\r\n" removal
//    - Null bytes
// 4. TestSanitizeHeaderValue_SizeLimit
//    - Exactly maxSize
//    - Over maxSize
// 5. TestInjectHeader_Protected
//    - Attempt to inject "Host"
//    - Verify skipped silently
// 6. TestInjectHeader_Collision_NoOverride
//    - Existing header + override=false
//    - Verify original preserved
// 7. TestInjectHeader_Collision_Override
//    - Existing header + override=true
//    - Verify replaced
// 8. TestInjectHeader_New
//    - Non-existent header
//    - Verify added
```

### Task 3.2: Integration Testing

**File**: `jwt_claims_headers_test.go`

```go
// Integration test checklist:
// 1. TestServeHTTP_ValidJWT
//    - Full request with valid JWT
//    - Verify all configured headers injected
// 2. TestServeHTTP_MissingJWT_ContinueOnError
//    - No Authorization header
//    - continueOnError=true
//    - Verify request passes through
// 3. TestServeHTTP_MissingJWT_StrictMode
//    - No Authorization header
//    - continueOnError=false
//    - Verify 401 response
// 4. TestServeHTTP_MalformedJWT_ContinueOnError
//    - Invalid JWT format
//    - Verify request passes through
// 5. TestServeHTTP_MalformedJWT_StrictMode
//    - Invalid JWT format
//    - Verify 401 response
// 6. TestServeHTTP_MultipleClaimMappings
//    - 4+ mappings
//    - Verify all injected correctly
// 7. TestServeHTTP_HeaderSectionReading
//    - Claims from JWT header (kid, alg)
// 8. TestServeHTTP_RemoveSourceHeader
//    - removeSourceHeader=true
//    - Verify Authorization header removed
```

**Test JWT Token** (use from README):
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwicm9sZXMiOlsiYWRtaW4iLCJ1c2VyIl0sImN1c3RvbSI6eyJ0ZW5hbnRfaWQiOiJ0ZW5hbnQtMTIzIn0sImlhdCI6MTUxNjIzOTAyMn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

### Task 3.3: Test Execution & Validation

**Commands**:
```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out

# Run with race detection
go test ./... -race

# Run specific test
go test -v -run TestParseJWT_Valid
```

**Success Criteria**:
- 100% of unit tests pass
- Test coverage ≥ 85% for all files
- No race conditions detected
- All edge cases covered

## Phase 4: Security Review

**Agent**: `security-engineer`
**Dependencies**: Phase 3 completed
**Traits**: `coordination/escalation-protocol`

### Task 4.1: Security Audit

**Focus Areas**:

1. **Header Injection Attacks**
   - Test: `"value\r\nX-Evil: injected\r\n"`
   - Test: `"value\nX-Evil: injected"`
   - Test: Control characters (0x00-0x1F)
   - Verify: All control chars removed by SanitizeHeaderValue()

2. **Protected Header Bypass**
   - Test: Case variations (`HOST`, `Host`, `host`)
   - Test: Prefix/suffix attacks (`X-Forwarded-For-Evil`)
   - Verify: Exact match on protected list

3. **Resource Exhaustion**
   - Test: Extremely deep claim paths (100+ levels)
   - Test: Large claim values (10MB+)
   - Test: Many claim mappings (1000+)
   - Verify: maxClaimDepth enforced
   - Verify: maxHeaderSize enforced

4. **Type Confusion**
   - Test: Array where object expected
   - Test: Object where primitive expected
   - Verify: Graceful error handling, no panics

5. **Concurrent Access**
   - Test: 1000 concurrent requests
   - Test: Shared state access patterns
   - Verify: No race conditions (go test -race)

**Security Test Suite**:
```bash
# Create security_test.go
# Run: go test -v -run Security
```

**Deliverables**:
- Security audit report (findings + recommendations)
- Additional security test cases
- Documentation of security considerations

### Task 4.2: Threat Modeling

**Threat Scenarios**:

| Threat | Mitigation | Verification |
|--------|-----------|--------------|
| Header injection via \r\n | Sanitize control chars | Test with \r\n sequences |
| Protected header override | Blacklist check | Test all protected headers |
| Memory exhaustion | Size limits | Test with 10MB+ values |
| Deep recursion DoS | Depth limit | Test 100+ nested levels |
| Type confusion panic | Type assertions | Test mixed types |

## Phase 5: Docker Testing Environment

**Agent**: `devops-engineer`
**Directory**: `examples/`
**Traits**: `safety/branch-check`

### Task 5.1: Docker Compose Setup

**File**: `examples/docker-compose.yml`

```yaml
version: '3.8'

services:
  traefik:
    image: traefik:v3.0
    command:
      - "--api.insecure=true"
      - "--providers.docker=true"
      - "--providers.file.directory=/etc/traefik/dynamic"
      - "--experimental.localPlugins.traefik-jwt-decoder-plugin.moduleName=github.com/user/traefik-jwt-decoder-plugin"
    ports:
      - "80:80"
      - "8080:8080"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "./dynamic-config.yml:/etc/traefik/dynamic/config.yml:ro"
      - "../:/plugins-local/src/github.com/user/traefik-jwt-decoder-plugin"
    networks:
      - traefik-net

  whoami:
    image: traefik/whoami:v1.10
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.whoami.rule=Host(`whoami.localhost`)"
      - "traefik.http.routers.whoami.middlewares=jwt-decoder@file"
    networks:
      - traefik-net

networks:
  traefik-net:
    driver: bridge
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
              override: false
            - claimPath: "email"
              headerName: "X-User-Email"
              override: false
            - claimPath: "roles"
              headerName: "X-User-Roles"
              override: false
              arrayFormat: "comma"
            - claimPath: "custom.tenant_id"
              headerName: "X-Tenant-Id"
              override: false
          sections:
            - "payload"
          continueOnError: true
          maxClaimDepth: 10
          maxHeaderSize: 8192
```

### Task 5.2: Manual Testing Script

**File**: `examples/test-plugin.sh`

```bash
#!/bin/bash
set -e

JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwicm9sZXMiOlsiYWRtaW4iLCJ1c2VyIl0sImN1c3RvbSI6eyJ0ZW5hbnRfaWQiOiJ0ZW5hbnQtMTIzIn0sImlhdCI6MTUxNjIzOTAyMn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

echo "=== Test 1: Valid JWT ==="
curl -H "Authorization: Bearer $JWT_TOKEN" http://whoami.localhost
echo -e "\n"

echo "=== Test 2: Missing JWT ==="
curl http://whoami.localhost
echo -e "\n"

echo "=== Test 3: Invalid JWT ==="
curl -H "Authorization: Bearer invalid.token.here" http://whoami.localhost
echo -e "\n"

echo "=== Test 4: No Bearer Prefix ==="
curl -H "Authorization: $JWT_TOKEN" http://whoami.localhost
echo -e "\n"
```

**Execution**:
```bash
cd examples
docker-compose up -d
sleep 5  # Wait for Traefik to load plugin
./test-plugin.sh
docker-compose logs traefik  # Check for errors
docker-compose down
```

## Phase 6: Documentation

**Agent**: `technical-writer`
**Traits**: `coordination/documentation-handoff`, `safety/context-verification`

### Task 6.1: API Documentation

**Files to Update**:
- `README.md` (already excellent, minor refinements)
- Create `ARCHITECTURE.md` (system design)
- Create `SECURITY.md` (security considerations)
- Create `CONTRIBUTING.md` (development guide)

### Task 6.2: Inline Code Documentation

**Requirements**:
- All exported functions have godoc comments
- Complex logic has inline explanations
- Security-critical sections clearly marked
- Examples in godoc format

**Example**:
```go
// ParseJWT decodes a JWT token without signature verification.
// This function is designed for scenarios where JWT validation occurs
// at the edge and internal services only need claim extraction.
//
// The token must be in the format: header.payload.signature
// Both header and payload are base64url-encoded JSON objects.
// The signature is stored but not validated.
//
// Example:
//   jwt, err := ParseJWT("eyJhbGc...")
//   if err != nil {
//       return nil, fmt.Errorf("parse failed: %w", err)
//   }
//   userID := jwt.Payload["sub"]
func ParseJWT(token string) (*JWT, error)
```

### Task 6.3: Usage Examples

**Create**: `examples/README.md`

Include:
- Quick start guide
- Configuration examples for common scenarios
- Troubleshooting section
- Performance tuning tips

## Phase 7: Git Workflow & Release

**Agent**: `git-helper`
**Traits**: `coordination/version-control-coordination`

### Task 7.1: Initial Commit

**Sequence**:
```bash
# 1. Review all changes
git status
git diff

# 2. Stage files in logical groups
git add go.mod go.sum .traefik.yml
git add config.go config_test.go
git add jwt.go jwt_test.go
git add claims.go claims_test.go
git add headers.go headers_test.go
git add jwt_claims_headers.go jwt_claims_headers_test.go

# 3. Stage documentation
git add README.md CLAUDE.md IMPLEMENTATION_PLAN.md
git add examples/

# 4. Create comprehensive commit
git commit -m "Initial implementation of JWT claims to headers middleware

Implemented:
- JWT parsing without signature verification
- Nested claim extraction with dot notation
- Secure header injection with sanitization
- Comprehensive test coverage (85%+)
- Docker Compose testing environment

Security features:
- Protected header blacklist
- Header injection attack prevention
- Size and depth limits
- Control character sanitization

🤖 Generated with Claude Code
Co-Authored-By: Claude <noreply@anthropic.com>"
```

### Task 7.2: Create Release

```bash
# Tag release
git tag -a v0.1.0 -m "Initial release"

# Push to remote (if configured)
git push origin main
git push origin v0.1.0
```

## Agent Coordination Matrix

| Phase | Primary Agent | Supporting Agents | Coordination Traits |
|-------|--------------|-------------------|-------------------|
| 0 | ai-researcher | - | enhancement/mcp-integration |
| 1A | go-engineer | qa-engineer | coordination/testing-handoff |
| 1B | go-engineer | qa-engineer | coordination/testing-handoff |
| 1C | go-engineer | qa-engineer | coordination/testing-handoff |
| 1D | security-engineer | qa-engineer | coordination/testing-handoff |
| 2 | go-engineer | qa-engineer | coordination/testing-handoff |
| 3 | qa-engineer | All | coordination/testing-handoff |
| 4 | security-engineer | qa-engineer | coordination/escalation-protocol |
| 5 | devops-engineer | - | safety/branch-check |
| 6 | technical-writer | All | coordination/documentation-handoff |
| 7 | git-helper | All | coordination/version-control-coordination |

## Parallel Execution Strategy

**Concurrent Streams** (Phase 1):
```
Stream 1A: jwt.go          ┐
Stream 1B: config.go       ├─→ Independent, execute in parallel
Stream 1C: claims.go       │   (4 simultaneous agents)
Stream 1D: headers.go      ┘

↓ Synchronization Point (all streams complete)

Phase 2: jwt_claims_headers.go (depends on 1A, 1B, 1C, 1D)
```

**Agent Invocation** (single message, multiple Task calls):
```
Task(agent="go-engineer", file="jwt.go", research_req=True)
Task(agent="go-engineer", file="config.go", research_req=True)
Task(agent="go-engineer", file="claims.go", research_req=True)
Task(agent="security-engineer", file="headers.go", research_req=True)
```

## Success Metrics

**Implementation Quality**:
- ✅ All tests pass (go test ./...)
- ✅ Test coverage ≥ 85%
- ✅ No race conditions (go test -race)
- ✅ No external dependencies (go.mod verification)
- ✅ Passes golangci-lint (if available)

**Security Validation**:
- ✅ Security audit report completed
- ✅ All OWASP header injection tests pass
- ✅ Protected header bypass attempts fail
- ✅ Resource exhaustion tests pass

**Integration Validation**:
- ✅ Loads successfully in Traefik via docker-compose
- ✅ Extracts claims correctly in manual tests
- ✅ Handles errors according to continueOnError config
- ✅ Logs appropriate messages at correct levels

**Documentation Completeness**:
- ✅ All exported functions have godoc
- ✅ README includes configuration examples
- ✅ SECURITY.md documents threat model
- ✅ Examples directory includes working docker-compose setup

## Critical Constraints & Reminders

**Yaegi Limitations**:
- NO external dependencies (only stdlib)
- Some stdlib packages may have limitations
- Cannot use cgo
- Cannot use unsafe package
- Limited reflection capabilities

**Thread Safety**:
- No shared mutable state
- All data flows through request context
- Consider using sync.Pool for performance if needed

**Error Handling Philosophy**:
- continueOnError=true: Log errors, pass request through
- continueOnError=false: Return 401 with JSON error body
- Never panic - always return errors gracefully

**Performance Considerations**:
- Parse JWT once, reuse for all claim extractions
- Avoid string concatenation in loops (use strings.Builder)
- Minimize memory allocations in hot path
- Consider pre-compiling claim paths if optimization needed

## Escalation Paths

**Technical Blockers** → `sr-architect`
**Security Concerns** → `security-engineer` → `sr-architect`
**Performance Issues** → `performance-engineer`
**Integration Problems** → `devops-engineer` → `integration-architect`

## Final Deliverables Checklist

- [ ] All .go implementation files complete
- [ ] All *_test.go files complete with ≥85% coverage
- [ ] go.mod verified (no external dependencies)
- [ ] .traefik.yml metadata file
- [ ] examples/docker-compose.yml working
- [ ] examples/test-plugin.sh executable
- [ ] README.md comprehensive
- [ ] SECURITY.md created
- [ ] Security audit report
- [ ] All files committed to git
- [ ] v0.1.0 release tagged

---

**Ready for Execution**: This plan is complete and agents can begin work. Start with Phase 0 (research) BEFORE any code implementation.
