# Security Policy

## Table of Contents
- [Overview](#overview)
- [Security Model](#security-model)
- [Threat Model](#threat-model)
- [Security Controls](#security-controls)
- [Known Limitations](#known-limitations)
- [Deployment Recommendations](#deployment-recommendations)
- [Security Testing](#security-testing)
- [Vulnerability Reporting](#vulnerability-reporting)

## Overview

The Traefik JWT Decoder Plugin is designed for **internal service-to-service communication** within trusted network boundaries. It extracts claims from JWT tokens and injects them as HTTP headers **without performing signature verification**.

**⚠️ CRITICAL SECURITY NOTICE**: This plugin does NOT validate JWT signatures. It is intended for use only behind a validated API gateway or authentication layer where JWT signature verification has already been performed.

## Security Model

### Trust Boundary

```
┌─────────────────────────────────────────────────────┐
│  Internet (Untrusted)                                │
│                                                       │
│  ┌──────────────────────────────────────────────┐   │
│  │  API Gateway / Auth Proxy                     │   │
│  │  - JWT Signature Verification ✓               │   │
│  │  - Authentication ✓                           │   │
│  │  - Rate Limiting ✓                            │   │
│  └──────────────────────────────────────────────┘   │
│                     │                                 │
│                     ▼                                 │
├─────────────────────────────────────────────────────┤
│  Internal Network (Trusted)                          │
│                                                       │
│  ┌──────────────────────────────────────────────┐   │
│  │  Traefik with JWT Decoder Plugin              │   │
│  │  - JWT Claim Extraction                       │   │
│  │  - Header Sanitization ✓                      │   │
│  │  - No Signature Verification ✗                │   │
│  └──────────────────────────────────────────────┘   │
│                     │                                 │
│                     ▼                                 │
│  ┌──────────────────────────────────────────────┐   │
│  │  Backend Services                             │   │
│  │  - Consume Headers                            │   │
│  │  - Business Logic Validation                  │   │
│  └──────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

### Security Assumptions

1. **Pre-Validated Tokens**: All JWTs have been validated by an upstream authentication layer
2. **Trusted Network**: Plugin operates within an internal network protected from external access
3. **Authorized Sources**: Only legitimate services send requests to this plugin
4. **Correct Configuration**: Operators configure the plugin according to security best practices

## Threat Model

### Threat Analysis Table

| Threat | Attack Vector | Impact | Mitigated | Mitigation Strategy |
|--------|--------------|--------|-----------|---------------------|
| **Header Injection** | CRLF in JWT claims | High | ✅ | Control character sanitization (0x00-0x1F, 0x7F removed) |
| **Protected Header Override** | Malicious claim→header mapping | Critical | ✅ | Case-insensitive protected header blocklist |
| **Memory Exhaustion** | Large claim values | High | ✅ | `maxHeaderSize` limit (default 8KB) |
| **CPU Exhaustion** | Deep claim nesting | High | ✅ | `maxClaimDepth` limit (default 10 levels) |
| **Type Confusion** | Unexpected JSON types | Medium | ✅ | Safe type assertion with error handling |
| **Unicode Normalization** | Unicode CRLF (U+000D, U+000A) | High | ✅ | All control characters removed regardless of encoding |
| **JWT Signature Bypass** | Forged/tampered tokens | Critical | ⚠️ | **MUST be mitigated at API gateway** |
| **Information Disclosure** | JWT in logs/errors | Medium | ⚠️ | Log only errors, not token contents |

### Attack Scenarios

#### 1. Header Injection Attack

**Scenario**: Attacker controls JWT claim with embedded CRLF to inject malicious headers

**Attack Payload**:
```json
{
  "email": "attacker@evil.com\r\nX-Admin: true\r\n"
}
```

**Mitigation**:
- `SanitizeHeaderValue()` removes all control characters (0x00-0x1F, 0x7F)
- Both ASCII and Unicode CRLF sequences stripped
- Tested with: `\r\n`, `\n`, `\r`, `\u000D\u000A`

**Validation**:
```bash
go test -v -run TestSecurity_UnicodeNormalizationAttack
go test -v -run TestSanitizeHeaderValue_HeaderInjection
```

#### 2. Protected Header Override

**Scenario**: Attacker configures claim mapping to override security-critical headers

**Attack Configuration**:
```yaml
claims:
  - claimPath: "sub"
    headerName: "X-Forwarded-For"  # Try to spoof source IP
```

**Mitigation**:
- `IsProtectedHeader()` blocks injection to protected headers (case-insensitive)
- Protected headers: `Host`, `X-Forwarded-*`, `X-Real-IP`, `Content-Length`, `Content-Type`, `Transfer-Encoding`
- Silently skips protected headers (no error, logged if verbose)

**Validation**:
```bash
go test -v -run TestSecurity_ProtectedHeaderBypass
go test -v -run TestSecurity_ProtectedHeaderIntegration
```

#### 3. Resource Exhaustion (DoS)

**Scenario A - Memory**: Attacker sends JWT with extremely large claim values

**Attack Payload**:
```json
{
  "data": "<10MB of 'A' characters>"
}
```

**Mitigation**:
- `maxHeaderSize` enforced (default 8KB)
- Size check before processing
- Returns error for oversized claims

**Scenario B - CPU**: Attacker sends JWT with deeply nested claims

**Attack Payload**:
```json
{
  "a": {"b": {"c": {"d": {"e": {"f": {"g": ... }}}}}}
}
```

**Mitigation**:
- `maxClaimDepth` enforced (default 10 levels)
- Depth check before navigation
- Returns error for excessively deep paths

**Validation**:
```bash
go test -v -run TestSecurity_LargeClaimValue
go test -v -run TestSecurity_DeepClaimPath
```

#### 4. Type Confusion

**Scenario**: Attacker sends JWT with unexpected claim types to trigger errors

**Attack Payload**:
```json
{
  "roles": [1, 2, 3],  // Expected strings
  "user": "string",     // Expected object
  "count": {"nested": "object"}  // Expected number
}
```

**Mitigation**:
- Safe type assertions with `ok` checks
- Graceful handling of nil values
- Array elements recursively converted with error handling
- Objects marshaled to JSON strings

**Validation**:
```bash
go test -v -run TestSecurity_TypeConfusion
go test -v -run TestSecurity_MixedTypeArrayConversion
```

## Security Controls

### 1. Input Sanitization

**Implementation**: `headers.go::SanitizeHeaderValue()`

**Controls**:
- Removes all ASCII control characters (0x00-0x1F)
- Removes DEL character (0x7F)
- Handles Unicode-encoded control characters (U+000D, U+000A, U+2028, U+2029)
- Enforces maximum header size limit
- Trims leading/trailing whitespace

**Code Reference**:
```go
sanitized := strings.Map(func(r rune) rune {
    if r < 0x20 || r == 0x7F {
        return -1  // Remove character
    }
    return r
}, value)
```

### 2. Protected Header Guard

**Implementation**: `headers.go::IsProtectedHeader()`

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

**Behavior**: Silently skips injection (not an error) to prevent configuration errors from breaking requests.

### 3. Resource Limits

**Configuration Parameters**:

| Parameter | Default | Purpose | Recommendation |
|-----------|---------|---------|----------------|
| `maxClaimDepth` | 10 | Prevent deep recursion | 5-20 depending on JWT structure |
| `maxHeaderSize` | 8192 (8KB) | Prevent memory exhaustion | 4KB-16KB depending on claim size |

**Enforcement Points**:
- Depth check in `ExtractClaim()` before navigation
- Size check in `SanitizeHeaderValue()` before processing

### 4. Error Handling

**Modes**:
- **Strict Mode** (`continueOnError: false`): Return 401 on any JWT error
- **Permissive Mode** (`continueOnError: true`): Log errors and pass request through

**Error Types**:
- Missing JWT token
- Invalid JWT format (wrong segment count)
- Invalid base64 encoding
- Invalid JSON structure
- Claim not found
- Claim depth exceeded
- Header size exceeded

**Response Format** (strict mode):
```json
{
  "error": "unauthorized",
  "message": "invalid JWT token"
}
```

### 5. Concurrency Safety

**Thread Safety**:
- No shared mutable state in plugin struct
- Each request processed independently
- All data flows through request context

**Validation**:
```bash
go test -race ./... -count=100
```

## Known Limitations

### 1. No Signature Verification ⚠️

**Risk Level**: CRITICAL

**Description**: Plugin does NOT verify JWT signatures. Any JWT with valid structure will be processed.

**Mitigation**:
- **MUST** deploy behind authenticated API gateway
- Gateway MUST verify JWT signatures before forwarding
- Use only within trusted network boundaries
- Never expose directly to internet

**Example Secure Architecture**:
```yaml
Internet → Kong/API Gateway (JWT verification) → Traefik (claim extraction) → Services
```

### 2. Trust All Claim Content

**Risk Level**: MEDIUM

**Description**: Plugin trusts all claim values after sanitization. It does not validate business logic.

**Mitigation**:
- Backend services MUST validate claims for business logic
- Do not rely solely on claim presence for authorization
- Implement additional validation layers in services

**Example**:
```go
// Backend service must validate
func authorizeAdmin(r *http.Request) error {
    role := r.Header.Get("X-User-Role")
    if role != "admin" {
        return errors.New("insufficient permissions")
    }
    // Additional validation: check against database, etc.
    return nil
}
```

### 3. Information Disclosure via Logs

**Risk Level**: LOW

**Description**: Plugin logs claim paths and header names (not values) for debugging.

**Mitigation**:
- Review logs for sensitive information disclosure
- Configure log levels appropriately for production
- Consider disabling verbose logging in production
- Implement log rotation and retention policies

**Example Log Output**:
```
[plugin-name] Claim not found: user.ssn  # OK - no value disclosed
[plugin-name] Injected header: X-User-ID = 12345  # Value logged - consider in production
```

### 4. No Rate Limiting

**Risk Level**: MEDIUM

**Description**: Plugin does not implement rate limiting or request throttling.

**Mitigation**:
- Implement rate limiting at API gateway level
- Use Traefik's built-in rate limiting middleware
- Monitor request patterns for anomalies

## Deployment Recommendations

### Minimum Security Requirements

✅ **REQUIRED**:
1. Deploy behind API gateway with JWT signature verification
2. Use within internal network (no direct internet exposure)
3. Configure `maxClaimDepth` and `maxHeaderSize` appropriately
4. Review protected headers list for your environment
5. Enable TLS for all communication channels

⚠️ **RECOMMENDED**:
1. Use strict mode (`continueOnError: false`) in production
2. Implement rate limiting at gateway level
3. Monitor plugin logs for security anomalies
4. Regularly update plugin to latest version
5. Conduct periodic security audits

❌ **PROHIBITED**:
1. Exposing plugin directly to internet
2. Using without upstream JWT verification
3. Trusting claim values for critical authorization without backend validation
4. Storing JWTs in logs or error messages

### Secure Configuration Example

**Note**: This example shows production-recommended values. Code defaults may differ (e.g., `continueOnError` defaults to `true`, but `false` is recommended for production; `maxClaimDepth` defaults to `10`, but `5` is recommended for stricter security).

```yaml
http:
  middlewares:
    jwt-claims-secure:
      plugin:
        traefik-jwt-decoder-plugin:
          sourceHeader: "Authorization"
          tokenPrefix: "Bearer "
          claims:
            - claimPath: "sub"
              headerName: "X-User-ID"
              override: false  # Don't override existing headers
            - claimPath: "email"
              headerName: "X-User-Email"
              override: false
            - claimPath: "roles"
              headerName: "X-User-Roles"
              arrayFormat: "comma"
          sections: ["payload"]  # Only read from payload (not header)
          continueOnError: false  # Strict mode for production
          removeSourceHeader: true  # Remove JWT after processing
          maxClaimDepth: 5  # Restrict nesting depth
          maxHeaderSize: 4096  # Limit header size (4KB)
```

For production deployment architecture including network design, Docker, Kubernetes, and air-gapped environments, see [DEPLOYMENT.md](DEPLOYMENT.md).

### Security Monitoring

Monitor these security-related metrics:
- JWT parse error rate (alert if > 5%)
- Header size/depth exceeded counts (potential DoS)
- Protected header injection attempts
- Unusual claim path patterns (reconnaissance)

## Security Testing

### Running Security Tests

**Execute full security test suite**:
```bash
go test -v -run TestSecurity ./...
```

**Individual test categories**:
```bash
# Header injection protection
go test -v -run TestSecurity_UnicodeNormalizationAttack
go test -v -run TestSanitizeHeaderValue_HeaderInjection

# Protected header bypass attempts
go test -v -run TestSecurity_ProtectedHeaderBypass
go test -v -run TestSecurity_ProtectedHeaderIntegration

# Resource exhaustion (DoS)
go test -v -run TestSecurity_DeepClaimPath
go test -v -run TestSecurity_LargeClaimValue
go test -v -run TestSecurity_ManyClaimMappings

# Type confusion and data handling
go test -v -run TestSecurity_TypeConfusion
go test -v -run TestSecurity_MixedTypeArrayConversion

# Repeated attack attempts
go test -v -run TestSecurity_RepeatedControlCharacters
```

**Race condition detection**:
```bash
go test -race ./... -count=100
```

**Test coverage analysis**:
```bash
go test -cover ./...
# Current coverage: 93%
```

### Security Test Matrix

| Attack Vector | Test Coverage | Pass Rate | Notes |
|---------------|--------------|-----------|-------|
| ASCII CRLF Injection | ✅ Comprehensive | 100% | Tests `\r`, `\n`, `\r\n` |
| Unicode CRLF | ✅ Comprehensive | 100% | Tests U+000D, U+000A, U+2028, U+2029 |
| Null Bytes | ✅ Comprehensive | 100% | All 0x00-0x1F control chars |
| DEL Character | ✅ Comprehensive | 100% | 0x7F removal |
| Protected Header Bypass | ✅ Comprehensive | 100% | 19 test cases covering variations |
| Deep Nesting (100 levels) | ✅ Comprehensive | 100% | Configurable depth limits |
| Large Claims (10MB) | ✅ Comprehensive | 100% | Size limit enforcement |
| Many Mappings (1000) | ✅ Comprehensive | 100% | No crashes or hangs |
| Type Confusion | ✅ Comprehensive | 100% | Array, object, primitive mismatches |
| Concurrent Access | ✅ Race Detector | 100% | No race conditions detected |

## Vulnerability Reporting

### Responsible Disclosure

If you discover a security vulnerability in this plugin, please follow responsible disclosure practices:

1. **DO NOT** open a public GitHub issue
2. **DO NOT** disclose the vulnerability publicly before patch is available
3. **DO** email security contact with details (see below)
4. **DO** allow reasonable time for patch development (typically 90 days)

### Reporting Process

**Email**: [Your Security Contact Email]

**Include**:
- Description of vulnerability
- Steps to reproduce
- Proof of concept (if applicable)
- Potential impact assessment
- Suggested remediation (if any)

**Expected Response Time**:
- Initial acknowledgment: 48 hours
- Severity assessment: 1 week
- Patch timeline: 2-12 weeks (depending on severity)

### Security Severity Levels

| Level | Criteria | Response Time |
|-------|----------|---------------|
| **Critical** | Remote code execution, authentication bypass affecting all users | 48 hours |
| **High** | Privilege escalation, information disclosure, DoS affecting service | 1 week |
| **Medium** | Limited information disclosure, localized DoS, configuration issues | 2 weeks |
| **Low** | Minor information disclosure, edge case bugs | 1 month |

### Security Updates

**Notification Channels**:
- GitHub Security Advisories
- Release notes in CHANGELOG.md
- Git tags with `security-` prefix

**Versioning for Security Patches**:
- Critical/High: Immediate patch release (e.g., v1.0.0 → v1.0.1)
- Medium: Next minor release (e.g., v1.0.x → v1.1.0)
- Low: Next major release (e.g., v1.x.x → v2.0.0)

## Security Checklist

**Before Deployment**:
- [ ] JWT signature verification configured at API gateway
- [ ] Plugin deployed within internal network only
- [ ] TLS enabled for all communication channels
- [ ] `continueOnError: false` in production configuration
- [ ] `maxClaimDepth` and `maxHeaderSize` configured appropriately
- [ ] Protected headers list reviewed for environment
- [ ] Rate limiting configured at gateway level
- [ ] Monitoring and alerting configured
- [ ] Security tests passing (`go test -v -run TestSecurity`)
- [ ] No race conditions (`go test -race ./...`)

**Regular Maintenance**:
- [ ] Review security logs monthly
- [ ] Update plugin to latest version quarterly
- [ ] Conduct security audit annually
- [ ] Review and update protected headers list as needed
- [ ] Test disaster recovery procedures semi-annually

## References

- [OWASP JWT Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html)
- [CWE-93: Improper Neutralization of CRLF Sequences](https://cwe.mitre.org/data/definitions/93.html)
- [CWE-400: Uncontrolled Resource Consumption](https://cwe.mitre.org/data/definitions/400.html)
- [Traefik Plugin Documentation](https://plugins.traefik.io/)
