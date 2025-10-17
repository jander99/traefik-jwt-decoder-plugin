# traefik-jwt-decoder-plugin

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://go.dev)
[![Test Coverage](https://img.shields.io/badge/coverage-93%25-brightgreen.svg)]()
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Traefik Plugin](https://img.shields.io/badge/traefik-plugin-blue.svg)](https://plugins.traefik.io/)

A Traefik middleware plugin that extracts claims from JWT tokens and injects them as HTTP headers for upstream services. No signature verification - pure claim extraction for internal service-to-service communication.

## Overview

This plugin decodes JWT tokens (without validation), extracts specified claims, and injects them as HTTP headers. It's designed for scenarios where JWT validation happens at the edge (API gateway) and internal services need access to JWT claims without re-parsing tokens.

**⚠️ SECURITY NOTICE**: This plugin does NOT verify JWT signatures. Deploy only behind authenticated API gateways. See [SECURITY.md](docs/SECURITY.md) for details.

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
      moduleName: github.com/jander99/traefik-jwt-decoder-plugin
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
      moduleName: github.com/jander99/traefik-jwt-decoder-plugin
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

          # Validation and logging (v0.1.0+)
          strictMode: false           # Validate JWT header has 'alg' field
          logMissingClaims: false     # Log warnings for missing claims
          logLevel: "warn"            # Options: "debug", "info", "warn", "error"
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
| `strictMode` | bool | `false` | Validate JWT header has 'alg' field (added in v0.1.0) |
| `logMissingClaims` | bool | `false` | Log warnings when claims are not found (added in v0.1.0) |
| `logLevel` | string | `"warn"` | Logging verbosity: `"debug"`, `"info"`, `"warn"`, `"error"` (added in v0.1.0) |

### Claim Mapping Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `claimPath` | string | Yes | Path to claim (dot notation for nested) |
| `headerName` | string | Yes | Target HTTP header name |
| `override` | bool | No (default: `false`) | Override existing header if present |
| `arrayFormat` | string | No (default: `"comma"`) | Array format: `"comma"` or `"json"` |

## Practical Examples

### Production Configuration (Recommended)

Minimal logging, strict validation disabled for compatibility:

```yaml
http:
  middlewares:
    jwt-decoder-prod:
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
          logLevel: "error"  # Only log errors, reduce noise
```

### Development Configuration

Verbose logging for debugging:

```yaml
http:
  middlewares:
    jwt-decoder-dev:
      plugin:
        traefik-jwt-decoder-plugin:
          sourceHeader: "Authorization"
          tokenPrefix: "Bearer "
          claims:
            - claimPath: "sub"
              headerName: "X-User-Id"
          continueOnError: true
          strictMode: true            # Validate JWT headers
          logMissingClaims: true      # Log when claims are missing
          logLevel: "debug"           # Log everything including header injections
```

### Strict Security Configuration

Enable all validation, fail on errors:

```yaml
http:
  middlewares:
    jwt-decoder-strict:
      plugin:
        traefik-jwt-decoder-plugin:
          sourceHeader: "Authorization"
          tokenPrefix: "Bearer "
          claims:
            - claimPath: "sub"
              headerName: "X-User-Id"
          continueOnError: false      # Return 401 on JWT errors
          strictMode: true            # Validate JWT structure
          removeSourceHeader: true    # Remove Authorization header
          logLevel: "warn"
```

### Log Level Behavior

| Level | What Gets Logged | Use Case |
|-------|------------------|----------|
| `debug` | All operations including header injections | Local development, troubleshooting |
| `info` | Significant operations | Staging environments |
| `warn` | Warnings and errors (default) | Production (recommended) |
| `error` | Errors only | High-traffic production |

**Impact**: At 1000 req/s with 3 claims per request:
- `debug`: ~259M log lines/day
- `warn`: ~10K log lines/day (errors/warnings only)

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

**Security Audit**: See [SECURITY.md](docs/SECURITY.md) for complete threat model, security controls, and deployment recommendations.

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
├── docs/                     # Documentation
│   ├── ARCHITECTURE.md       # System design and data flow
│   ├── SECURITY.md           # Threat model and security controls
│   ├── CONTRIBUTING.md       # Development guidelines
│   └── CHANGELOG.md          # Version history
└── README.md                 # This file
```

### Common Pitfalls

- **Base64 Encoding**: JWTs use `base64.RawURLEncoding` (no padding)
- **Header Case**: HTTP headers are case-insensitive, always normalize
- **Type Assertions**: Claims are `map[string]interface{}`, check types before assertions
- **Nil Handling**: JWT claims may be null, handle gracefully

For detailed development guidelines, see [CONTRIBUTING.md](docs/CONTRIBUTING.md).

## Documentation

- **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** - System design, components, and data flow
- **[SECURITY.md](docs/SECURITY.md)** - Security model, threat analysis, and deployment recommendations
- **[CONTRIBUTING.md](docs/CONTRIBUTING.md)** - Development workflow, testing requirements, and PR guidelines
- **[CHANGELOG.md](docs/CHANGELOG.md)** - Version history and release notes
- **[examples/README.md](examples/README.md)** - Docker Compose testing environment setup
- **[CLAUDE.md](CLAUDE.md)** - Development guidance for Claude Code AI assistant

### API Documentation

Generate godoc locally:

```bash
godoc -http=:6060
# Visit http://localhost:6060/pkg/traefik_jwt_decoder_plugin/
```

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](docs/CONTRIBUTING.md) for:

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