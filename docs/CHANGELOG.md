# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned Features
- Optional JWT signature verification (HMAC, RSA, ECDSA)
- Claim value transformations (base64, templates, regex)
- Conditional injection based on claim values
- Multiple source header support
- Performance optimizations (claim path caching)
- Prometheus metrics integration

## [v0.1.0] - 2025-10-12

### Added

#### Core Functionality
- **JWT Parsing**: Base64url decoding of JWT tokens without signature verification
- **Claim Extraction**: Navigate nested claims using dot notation (e.g., `user.profile.email`)
- **Header Injection**: Inject extracted claims as HTTP headers for upstream services
- **Configuration Validation**: Comprehensive validation of plugin configuration
- **Multi-Section Support**: Read claims from JWT header and/or payload sections

#### Features
- **Array Support**: Handle array claims with configurable formatting:
  - Comma-separated: `["admin", "user"]` → `"admin, user"`
  - JSON string: `["admin", "user"]` → `["admin","user"]`
- **Error Handling Modes**:
  - Strict mode (`continueOnError: false`): Return 401 on JWT errors
  - Permissive mode (`continueOnError: true`): Pass requests through on errors
- **Header Collision Control**: Configurable override behavior for existing headers
- **Source Header Removal**: Optional removal of Authorization header after processing
- **Configurable Limits**:
  - `maxClaimDepth`: Maximum depth for nested claim paths (default: 10)
  - `maxHeaderSize`: Maximum header value size in bytes (default: 8192)

#### Security
- **Header Injection Prevention**: CRLF injection attack prevention via control character removal (0x00-0x1F, 0x7F)
- **Protected Header Blacklist**: Prevents modification of security-critical headers:
  - `host`, `x-forwarded-for`, `x-forwarded-host`, `x-forwarded-proto`, `x-forwarded-port`
  - `x-real-ip`, `content-length`, `content-type`, `transfer-encoding`
- **Resource Limits**: DoS prevention via depth and size limits
- **Type Safety**: Graceful handling of unexpected claim types
- **Thread Safety**: No shared mutable state, safe for concurrent requests

#### Testing
- **93% Test Coverage**: Comprehensive test suite covering:
  - Unit tests for all components (jwt, claims, headers, config)
  - Integration tests for full request/response cycle
  - Security tests for attack scenarios (header injection, DoS, type confusion)
  - Race condition tests (verified with `-race` flag)
- **Test Scenarios**:
  - Valid and invalid JWT parsing
  - Nested claim extraction with dot notation
  - Array claim handling (comma-separated and JSON)
  - Protected header blocking
  - Header sanitization (control characters, size limits)
  - Concurrent request handling
  - Error handling modes (strict and permissive)

#### Documentation
- **README.md**: Comprehensive project overview, configuration guide, and quick start
- **ARCHITECTURE.md**: System design, component breakdown, and data flow diagrams
- **SECURITY.md**: Complete threat model, security controls, and deployment recommendations
- **CONTRIBUTING.md**: Development workflow, coding standards, and testing requirements
- **examples/README.md**: Docker Compose testing environment with automated tests
- **Inline Documentation**: Godoc comments for all exported functions and types

#### Development
- **Docker Compose Testing Environment**: Complete testing setup with:
  - Traefik v3.0 with local plugin support
  - Whoami echo service for header inspection
  - Dynamic configuration for middleware setup
  - Automated test script with multiple scenarios
- **No External Dependencies**: Pure Go stdlib implementation (Traefik Yaegi compatible)

### Security
- **Security Audit Completed**: Comprehensive security review conducted
- **Attack Scenarios Tested**:
  - ASCII and Unicode CRLF injection (U+000D, U+000A, U+2028, U+2029)
  - Null byte injection (0x00-0x1F control characters)
  - DEL character injection (0x7F)
  - Protected header bypass attempts (19 test cases)
  - Deep nesting attacks (100 levels tested)
  - Large claim values (10MB tested)
  - Many claim mappings (1000 tested)
  - Type confusion with mixed types
  - Concurrent access (race detector passed)

### Performance
- **Minimal Overhead**: ~50-100μs per request for 5 claims on modern hardware
- **Time Complexity**:
  - JWT parsing: O(n) where n = JWT size
  - Claim extraction: O(d) where d = claim path depth
  - Overall request: O(n + c*d + c*m) where c = claim count, m = header value length
- **Memory Efficiency**:
  - ~10KB per request
  - No memory leaks
  - Efficient string operations with `strings.Builder`

### Known Limitations
- **No JWT Signature Verification**: By design - intended for internal networks with upstream verification
- **No Array Index Support**: Cannot extract specific array elements (e.g., `roles[0]`)
- **No Claim Value Transformation**: Claims injected as-is without transformation
- **No Conditional Injection**: All configured claims always attempted for injection
- **Single Source Header**: Only one source header supported (default: Authorization)

### Fixed
- N/A (initial release)

### Changed
- N/A (initial release)

### Deprecated
- N/A (initial release)

### Removed
- N/A (initial release)

---

## Version History

### [v0.1.0] - 2025-10-12
Initial production-ready release with core functionality, comprehensive security controls, and 93% test coverage.

---

## Upgrade Guides

### Upgrading to 0.1.0
This is the initial release - no upgrade path needed.

---

## Release Notes

### 0.1.0 Release Notes (2025-10-12)

**Traefik JWT Decoder Plugin v0.1.0** is now production-ready!

This initial release provides a lightweight, secure middleware for extracting JWT claims and injecting them as HTTP headers in internal service-to-service communication.

**Key Highlights**:
- **Security-First Design**: Comprehensive security controls with 93% test coverage
- **Zero Dependencies**: Pure Go stdlib implementation compatible with Traefik's Yaegi interpreter
- **High Performance**: <100μs overhead per request with minimal memory footprint
- **Flexible Configuration**: Support for nested claims, arrays, error modes, and collision handling
- **Production-Ready**: Complete documentation, security audit, and Docker Compose testing environment

**Important Security Notice**:
This plugin does NOT perform JWT signature verification. It is designed for internal networks where signature validation occurs at the API gateway. See SECURITY.md for deployment recommendations.

**Getting Started**:
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
          continueOnError: true
```

**Documentation**:
- [README.md](README.md) - Quick start and configuration
- [ARCHITECTURE.md](ARCHITECTURE.md) - System design
- [SECURITY.md](SECURITY.md) - Threat model and security controls
- [CONTRIBUTING.md](CONTRIBUTING.md) - Development guidelines

**Feedback Welcome**:
Please report issues, feature requests, and security vulnerabilities through GitHub. See CONTRIBUTING.md for details.

