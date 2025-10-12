package traefik_jwt_decoder_plugin

import (
	"testing"
)

// TestParseJWT_Valid verifies successful parsing of valid JWT tokens
func TestParseJWT_Valid(t *testing.T) {
	// Test token from CLAUDE.md
	// Payload: {"sub":"1234567890","email":"test@example.com","roles":["admin","user"],"custom":{"tenant_id":"tenant-123"},"iat":1516239022}
	validToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwicm9sZXMiOlsiYWRtaW4iLCJ1c2VyIl0sImN1c3RvbSI6eyJ0ZW5hbnRfaWQiOiJ0ZW5hbnQtMTIzIn0sImlhdCI6MTUxNjIzOTAyMn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	jwt, err := ParseJWT(validToken)
	if err != nil {
		t.Fatalf("ParseJWT() failed with valid token: %v", err)
	}

	// Verify header
	if jwt.Header["alg"] != "HS256" {
		t.Errorf("Header alg = %v, want HS256", jwt.Header["alg"])
	}
	if jwt.Header["typ"] != "JWT" {
		t.Errorf("Header typ = %v, want JWT", jwt.Header["typ"])
	}

	// Verify payload
	if jwt.Payload["sub"] != "1234567890" {
		t.Errorf("Payload sub = %v, want 1234567890", jwt.Payload["sub"])
	}
	if jwt.Payload["email"] != "test@example.com" {
		t.Errorf("Payload email = %v, want test@example.com", jwt.Payload["email"])
	}

	// Verify nested claim
	custom, ok := jwt.Payload["custom"].(map[string]interface{})
	if !ok {
		t.Fatal("Payload custom is not a map")
	}
	if custom["tenant_id"] != "tenant-123" {
		t.Errorf("Payload custom.tenant_id = %v, want tenant-123", custom["tenant_id"])
	}

	// Verify signature (stored as-is)
	expectedSig := "SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	if jwt.Signature != expectedSig {
		t.Errorf("Signature = %v, want %v", jwt.Signature, expectedSig)
	}
}

// TestParseJWT_InvalidSegmentCount verifies error handling for wrong segment count
func TestParseJWT_InvalidSegmentCount(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		segments int
	}{
		{
			name:     "one segment",
			token:    "onlyonesegment",
			segments: 1,
		},
		{
			name:     "two segments",
			token:    "segment1.segment2",
			segments: 2,
		},
		{
			name:     "four segments",
			token:    "seg1.seg2.seg3.seg4",
			segments: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwt, err := ParseJWT(tt.token)
			if err == nil {
				t.Errorf("ParseJWT() expected error for %d segments, got nil", tt.segments)
			}
			if jwt != nil {
				t.Errorf("ParseJWT() expected nil JWT, got %v", jwt)
			}
		})
	}
}

// TestParseJWT_InvalidBase64 verifies error handling for invalid base64 encoding
func TestParseJWT_InvalidBase64(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "invalid base64 in header",
			token: "invalid!!!.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature",
		},
		{
			name:  "invalid base64 in payload",
			token: "eyJhbGciOiJIUzI1NiJ9.invalid!!!.signature",
		},
		{
			name:  "non-base64url characters in header",
			token: "abc@#$.eyJzdWIiOiIxMjM0In0.sig",
		},
		{
			name:  "non-base64url characters in payload",
			token: "eyJhbGciOiJIUzI1NiJ9.xyz@#$.sig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwt, err := ParseJWT(tt.token)
			if err == nil {
				t.Errorf("ParseJWT() expected error for invalid base64, got nil")
			}
			if jwt != nil {
				t.Errorf("ParseJWT() expected nil JWT, got %v", jwt)
			}
		})
	}
}

// TestParseJWT_InvalidJSON verifies error handling for invalid JSON content
func TestParseJWT_InvalidJSON(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "invalid JSON in header",
			token: "bm90IGpzb24.eyJzdWIiOiIxMjMifQ.sig", // "not json" base64url encoded
		},
		{
			name:  "invalid JSON in payload",
			token: "eyJhbGciOiJIUzI1NiJ9.bm90IGpzb24.sig", // "not json" base64url encoded
		},
		{
			name:  "truncated JSON in header",
			token: "eyJhbGci.eyJzdWIiOiIxMjMifQ.sig", // Truncated JSON
		},
		{
			name:  "truncated JSON in payload",
			token: "eyJhbGciOiJIUzI1NiJ9.eyJzdWI.sig", // Truncated JSON
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwt, err := ParseJWT(tt.token)
			if err == nil {
				t.Errorf("ParseJWT() expected error for invalid JSON, got nil")
			}
			if jwt != nil {
				t.Errorf("ParseJWT() expected nil JWT, got %v", jwt)
			}
		})
	}
}

// TestParseJWT_EmptyToken verifies error handling for empty token
func TestParseJWT_EmptyToken(t *testing.T) {
	jwt, err := ParseJWT("")
	if err == nil {
		t.Errorf("ParseJWT() expected error for empty token, got nil")
	}
	if jwt != nil {
		t.Errorf("ParseJWT() expected nil JWT, got %v", jwt)
	}
}

// TestExtractToken_WithPrefix verifies prefix removal
func TestExtractToken_WithPrefix(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		prefix   string
		expected string
	}{
		{
			name:     "standard bearer token",
			value:    "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			prefix:   "Bearer ",
			expected: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:     "bearer without space prefix",
			value:    "BearereyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			prefix:   "Bearer",
			expected: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:     "custom prefix",
			value:    "JWT:mytoken123",
			prefix:   "JWT:",
			expected: "mytoken123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractToken(tt.value, tt.prefix)
			if result != tt.expected {
				t.Errorf("ExtractToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestExtractToken_WithoutPrefix verifies behavior when prefix not present
func TestExtractToken_WithoutPrefix(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		prefix   string
		expected string
	}{
		{
			name:     "no prefix in value",
			value:    "plaintoken123",
			prefix:   "Bearer ",
			expected: "plaintoken123",
		},
		{
			name:     "different prefix in value",
			value:    "JWT:token123",
			prefix:   "Bearer ",
			expected: "JWT:token123",
		},
		{
			name:     "partial match",
			value:    "Beartoken",
			prefix:   "Bearer ",
			expected: "Beartoken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractToken(tt.value, tt.prefix)
			if result != tt.expected {
				t.Errorf("ExtractToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestExtractToken_EmptyPrefix verifies behavior with empty prefix
func TestExtractToken_EmptyPrefix(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "empty prefix returns value as-is",
			value:    "Bearer token123",
			expected: "Bearer token123",
		},
		{
			name:     "empty prefix with plain token",
			value:    "token123",
			expected: "token123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractToken(tt.value, "")
			if result != tt.expected {
				t.Errorf("ExtractToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestExtractToken_ExtraWhitespace verifies whitespace trimming
func TestExtractToken_ExtraWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		prefix   string
		expected string
	}{
		{
			name:     "extra space after prefix",
			value:    "Bearer  token123",
			prefix:   "Bearer ",
			expected: "token123",
		},
		{
			name:     "trailing whitespace",
			value:    "Bearer token123  ",
			prefix:   "Bearer ",
			expected: "token123",
		},
		{
			name:     "leading and trailing whitespace",
			value:    "Bearer   token123   ",
			prefix:   "Bearer ",
			expected: "token123",
		},
		{
			name:     "tab characters",
			value:    "Bearer \ttoken123\t",
			prefix:   "Bearer ",
			expected: "token123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractToken(tt.value, tt.prefix)
			if result != tt.expected {
				t.Errorf("ExtractToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}
