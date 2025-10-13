# Contributing to Traefik JWT Decoder Plugin

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Security Considerations](#security-considerations)
- [Documentation](#documentation)
- [Getting Help](#getting-help)

## Code of Conduct

### Our Pledge

We are committed to providing a welcoming and inclusive environment for all contributors, regardless of experience level, background, or identity.

### Expected Behavior

- Be respectful and considerate in communication
- Accept constructive criticism gracefully
- Focus on what is best for the project and community
- Show empathy towards other community members

### Unacceptable Behavior

- Harassment, discrimination, or offensive comments
- Trolling, insulting, or derogatory remarks
- Publishing others' private information
- Any conduct that could reasonably be considered inappropriate

## Getting Started

### Prerequisites

- **Go 1.21+**: Required for development
- **Docker & Docker Compose**: For integration testing
- **Git**: For version control
- **golangci-lint** (optional): For linting

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/traefik-jwt-decoder-plugin.git
   cd traefik-jwt-decoder-plugin
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/ORIGINAL_OWNER/traefik-jwt-decoder-plugin.git
   ```

## Development Setup

### Install Dependencies

```bash
# Verify Go installation
go version

# Download dependencies (none expected - stdlib only)
go mod tidy

# Verify project builds
go build ./...
```

### Verify Test Suite

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test -cover ./...

# Run with race detection
go test -race ./...
```

### Optional: Install golangci-lint

```bash
# macOS
brew install golangci-lint

# Linux
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Run linter
golangci-lint run
```

## Development Workflow

### Branch Naming

Use descriptive branch names with prefixes:

- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Test improvements
- `security/` - Security fixes

**Examples**:
- `feature/add-regex-claim-matching`
- `fix/handle-null-array-elements`
- `docs/update-architecture-diagram`
- `security/prevent-xxe-attack`

### Development Cycle

1. **Create Feature Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Changes**
   - Write code following [coding standards](#coding-standards)
   - Add or update tests
   - Update documentation

3. **Run Tests Locally**
   ```bash
   # Unit tests
   go test ./... -v

   # Coverage check (must be ≥85%)
   go test -cover ./...

   # Race detection
   go test -race ./...

   # Security tests
   go test -v -run TestSecurity ./...
   ```

4. **Commit Changes**
   ```bash
   git add .
   git commit -m "Add feature: descriptive commit message"
   ```

5. **Keep Branch Updated**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

6. **Push and Create Pull Request**
   ```bash
   git push origin feature/your-feature-name
   ```

### Commit Message Guidelines

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Test additions or changes
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `security`: Security fixes
- `chore`: Maintenance tasks

**Examples**:
```
feat(claims): add support for array index notation

Add ability to extract specific array elements using bracket notation.
Example: roles[0] extracts first role from array.

Closes #42
```

```
fix(headers): prevent panic on nil claim values

Add nil check before type assertion to prevent runtime panic
when JWT contains null claim values.

Fixes #56
```

```
security(sanitize): add Unicode control character filtering

Extend sanitization to handle Unicode control characters (U+2028, U+2029)
in addition to ASCII control characters.

Addresses security advisory SA-2024-001
```

## Coding Standards

### Go Style Guide

Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments):

- Use `gofmt` for formatting (automatically applied by editor)
- Follow Go naming conventions (camelCase for unexported, PascalCase for exported)
- Keep functions short and focused (single responsibility)
- Avoid package-level mutable state

### Code Organization

```go
// Package comment (required for godoc)
package traefik_jwt_decoder_plugin

// Imports grouped: stdlib, external, internal
import (
    "encoding/json"    // stdlib
    "fmt"
    "strings"
)

// Constants
const (
    DefaultMaxDepth = 10
    DefaultMaxSize  = 8192
)

// Types (with godoc comments)
type Config struct { ... }

// Exported functions (with godoc comments and examples)
func ParseJWT(token string) (*JWT, error) { ... }

// Unexported helper functions
func extractSegment(token string, index int) string { ... }
```

### Documentation Requirements

**All exported items must have godoc comments**:

```go
// ParseJWT decodes a JWT token without signature verification.
// This function is designed for scenarios where JWT validation occurs
// at the edge and internal services only need claim extraction.
//
// The token must be in the format: header.payload.signature
// Both header and payload are base64url-encoded JSON objects.
//
// Security Note: This function does NOT verify JWT signatures.
// It should only be used in trusted internal networks where
// signature validation happens at the API gateway.
//
// Example:
//   jwt, err := ParseJWT("eyJhbGc...")
//   if err != nil {
//       return nil, fmt.Errorf("parse failed: %w", err)
//   }
//   userID := jwt.Payload["sub"]
//
// Returns an error if the token format is invalid, base64 decoding
// fails, or JSON parsing fails.
func ParseJWT(token string) (*JWT, error)
```

### Error Handling

- Return descriptive errors with context
- Use `fmt.Errorf` with `%w` for error wrapping
- Never panic (use error returns instead)
- Log errors appropriately

**Good**:
```go
if err := validateConfig(cfg); err != nil {
    return nil, fmt.Errorf("configuration validation failed: %w", err)
}
```

**Bad**:
```go
if err := validateConfig(cfg); err != nil {
    panic(err)  // Never panic!
}
```

### Performance Considerations

- Minimize allocations in hot paths
- Use `strings.Builder` for string concatenation
- Pre-size slices when capacity is known
- Avoid unnecessary type conversions

**Good**:
```go
var builder strings.Builder
builder.Grow(estimatedSize)
for _, part := range parts {
    builder.WriteString(part)
}
return builder.String()
```

**Bad**:
```go
result := ""
for _, part := range parts {
    result += part  // Allocates on each iteration
}
return result
```

## Testing Requirements

### Test Coverage

**Minimum Requirements**:
- Overall test coverage: **≥85%**
- Security-critical functions: **100%**
- New features must include tests
- Bug fixes must include regression tests

### Test Structure

```go
func TestFunctionName_Scenario(t *testing.T) {
    // Arrange
    input := "test-input"
    expected := "expected-output"

    // Act
    result, err := FunctionName(input)

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("got %q, want %q", result, expected)
    }
}
```

### Test Categories

#### 1. Unit Tests

Test individual functions in isolation:

```bash
# Run unit tests
go test ./... -v

# Specific test
go test -v -run TestParseJWT_Valid
```

#### 2. Integration Tests

Test full request/response cycles:

```bash
# Run integration tests
go test -v -run Integration ./...
```

#### 3. Security Tests

Test security controls and attack scenarios:

```bash
# Run security test suite
go test -v -run TestSecurity ./...

# Specific security test
go test -v -run TestSecurity_HeaderInjection
```

#### 4. Race Condition Tests

Verify thread safety:

```bash
# Run with race detector
go test -race ./...

# Multiple iterations for reliability
go test -race ./... -count=100
```

### Test Data

Use realistic test data:

```go
// Good: Real JWT token for testing
const testJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

// Bad: Fake or invalid test data
const testJWT = "fake.token.here"
```

### Running the Full Test Suite

```bash
# Before submitting PR
./run-tests.sh  # Or manually:

go test ./... -v                          # All tests
go test -cover ./...                      # With coverage
go test -race ./... -count=10             # Race detection
go test -v -run TestSecurity ./...        # Security tests
golangci-lint run                         # Linting
```

## Pull Request Process

### Before Submitting

1. **Run Full Test Suite**
   ```bash
   go test ./... -v
   go test -cover ./...
   go test -race ./...
   ```

2. **Check Code Coverage**
   ```bash
   go test -coverprofile=coverage.out ./...
   go tool cover -func=coverage.out | tail -1
   # Should show ≥85% coverage
   ```

3. **Lint Code**
   ```bash
   golangci-lint run
   ```

4. **Update Documentation**
   - Update README.md if needed
   - Add godoc comments for new functions
   - Update CHANGELOG.md

5. **Test with Docker Compose**
   ```bash
   cd examples
   docker-compose up -d
   ./test-plugin.sh
   docker-compose down
   ```

### PR Description Template

```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix (non-breaking change fixing an issue)
- [ ] New feature (non-breaking change adding functionality)
- [ ] Breaking change (fix or feature causing existing functionality to break)
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Security tests added/updated
- [ ] Manual testing completed
- [ ] Test coverage ≥85%

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Comments added for complex logic
- [ ] Documentation updated
- [ ] No new warnings generated
- [ ] Tests pass locally
- [ ] CHANGELOG.md updated

## Related Issues
Closes #<issue_number>
```

### Review Process

1. **Automated Checks**: CI/CD runs tests automatically
2. **Code Review**: At least one maintainer reviews
3. **Security Review**: Required for security-sensitive changes
4. **Approval**: Maintainer approves and merges

### Addressing Feedback

- Respond to all review comments
- Make requested changes promptly
- Push updates to the same branch
- Request re-review when ready

## Security Considerations

### Security-Critical Code

Extra scrutiny required for:

- JWT parsing logic
- Claim extraction and navigation
- Header injection and sanitization
- Configuration validation
- Error handling

### Security Testing

All security changes must include:

1. **Threat Model Update**: Document new attack vectors
2. **Security Tests**: Comprehensive test coverage
3. **Security Review**: Maintainer security audit
4. **Documentation**: Update SECURITY.md

### Reporting Security Vulnerabilities

**DO NOT** open public GitHub issues for security vulnerabilities.

Instead:
1. Email security contact with details
2. Include reproduction steps
3. Suggest potential mitigations
4. Allow time for patch development

See [SECURITY.md](SECURITY.md) for full disclosure policy.

## Documentation

### Types of Documentation

1. **Code Documentation**: Godoc comments for all exported items
2. **README.md**: High-level overview and quick start
3. **ARCHITECTURE.md**: System design and component details
4. **SECURITY.md**: Threat model and security controls
5. **CONTRIBUTING.md**: This file
6. **examples/README.md**: Testing environment setup

### Documentation Standards

- Write for developers new to the project
- Include code examples where helpful
- Keep documentation in sync with code
- Use diagrams for complex concepts

### Generating Documentation

```bash
# Generate and view godoc
godoc -http=:6060
# Visit http://localhost:6060/pkg/traefik_jwt_decoder_plugin/
```

## Getting Help

### Communication Channels

- **GitHub Issues**: Bug reports, feature requests
- **Pull Requests**: Code discussions, reviews
- **GitHub Discussions**: General questions, ideas

### Best Practices for Asking Questions

1. **Search first**: Check existing issues and documentation
2. **Be specific**: Provide context, code samples, error messages
3. **Be patient**: Maintainers are volunteers
4. **Be respectful**: Follow code of conduct

### Issue Templates

Use provided issue templates for:
- Bug reports
- Feature requests
- Security vulnerabilities (private reporting only)

## Recognition

Contributors are recognized in:
- Git commit history
- CHANGELOG.md (for significant contributions)
- GitHub Contributors page

Thank you for contributing to the Traefik JWT Decoder Plugin!

---

**Last Updated**: 2025-10-12
**Project Version**: 1.0.0
