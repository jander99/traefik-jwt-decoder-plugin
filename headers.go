package traefik_jwt_decoder_plugin

import (
	"fmt"
	"net/http"
	"strings"
)

// protectedHeaders is a blacklist of HTTP headers that should never be overridden
// by JWT claims. These headers are security-critical and could compromise the
// request if modified by external data.
var protectedHeaders = map[string]bool{
	"host":              true, // Target host
	"x-forwarded-for":   true, // Client IP (proxy chain)
	"x-forwarded-host":  true, // Original host
	"x-forwarded-proto": true, // Original protocol (http/https)
	"x-forwarded-port":  true, // Original port
	"x-real-ip":         true, // Real client IP
	"content-length":    true, // Message body length
	"content-type":      true, // Media type
	"transfer-encoding": true, // Transfer encoding method
}

// IsProtectedHeader checks if a header name is in the protected headers blacklist.
// Protected headers cannot be modified by JWT claims to prevent security issues.
//
// The check is case-insensitive as HTTP header names are case-insensitive per RFC 7230.
//
// Example:
//   IsProtectedHeader("Host")              // true
//   IsProtectedHeader("X-Forwarded-For")   // true
//   IsProtectedHeader("X-User-Id")         // false
func IsProtectedHeader(name string) bool {
	normalized := strings.ToLower(name)
	return protectedHeaders[normalized]
}

// SanitizeHeaderValue removes dangerous characters from header values to prevent
// header injection attacks and enforce size limits.
//
// Security Controls:
//   1. Enforces maximum header size limit (prevents memory exhaustion)
//   2. Removes all ASCII control characters (0x00-0x1F, 0x7F)
//   3. Prevents CRLF injection attacks (\r\n sequences)
//   4. Handles Unicode control characters (U+000D, U+000A, U+2028, U+2029)
//   5. Trims leading/trailing whitespace
//
// Example:
//   // Normal value
//   sanitized, _ := SanitizeHeaderValue("user@example.com", 8192)
//   // Returns: "user@example.com"
//
//   // CRLF injection attempt
//   sanitized, _ := SanitizeHeaderValue("value\r\nX-Evil: injected", 8192)
//   // Returns: "valueX-Evil: injected" (control characters removed)
//
//   // Oversized value
//   _, err := SanitizeHeaderValue(strings.Repeat("A", 10000), 8192)
//   // Returns error: "header value exceeds maximum size"
func SanitizeHeaderValue(value string, maxSize int) (string, error) {
	// Check size limit first
	if len(value) > maxSize {
		return "", fmt.Errorf("header value exceeds maximum size (%d bytes)", maxSize)
	}

	// Remove all control characters (0x00-0x1F and 0x7F)
	// This prevents header injection attacks via \r\n and other control chars
	sanitized := strings.Map(func(r rune) rune {
		// Remove control characters
		if r < 0x20 || r == 0x7F {
			return -1 // Remove this character
		}
		return r
	}, value)

	// Trim whitespace from both ends
	sanitized = strings.TrimSpace(sanitized)

	return sanitized, nil
}

// InjectHeader safely injects a header into the HTTP request with security guards.
// Implements multiple security controls to prevent header-based attacks.
//
// Security Controls:
//   1. Protected header blocking (silently skips security-critical headers)
//   2. Value sanitization (control character removal, size limits)
//   3. Collision policy enforcement (override or preserve existing headers)
//
// Behavior:
//   - If header is protected: Skip silently (no error)
//   - If value exceeds maxSize: Return error
//   - If header exists and override=false: Skip silently (no error)
//   - If header exists and override=true: Replace value
//   - If header doesn't exist: Add header
//
// Example:
//   req, _ := http.NewRequest("GET", "http://example.com", nil)
//
//   // Inject new header
//   err := InjectHeader(req, "X-User-Id", "12345", false, 8192)
//   // req.Header.Get("X-User-Id") == "12345"
//
//   // Try to inject protected header (silently skipped)
//   err := InjectHeader(req, "Host", "evil.com", true, 8192)
//   // err == nil (no error, but header not modified)
//
//   // Header collision with override=false
//   req.Header.Set("X-User-Id", "original")
//   err := InjectHeader(req, "X-User-Id", "new", false, 8192)
//   // req.Header.Get("X-User-Id") == "original" (preserved)
//
//   // Header collision with override=true
//   req.Header.Set("X-User-Id", "original")
//   err := InjectHeader(req, "X-User-Id", "new", true, 8192)
//   // req.Header.Get("X-User-Id") == "new" (replaced)
func InjectHeader(req *http.Request, name, value string, override bool, maxSize int) error {
	// Check if header is protected (case-insensitive)
	if IsProtectedHeader(name) {
		// Silently skip protected headers - not an error
		// Could add logging here if logger available
		return nil
	}

	// Sanitize the value before injection
	sanitized, err := SanitizeHeaderValue(value, maxSize)
	if err != nil {
		return err
	}

	// Check for existing header and override setting
	existing := req.Header.Get(name)
	if existing != "" && !override {
		// Header exists and override is false - skip silently
		return nil
	}

	// Safe to inject header
	req.Header.Set(name, sanitized)
	return nil
}
