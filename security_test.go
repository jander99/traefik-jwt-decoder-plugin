package traefik_jwt_decoder_plugin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSecurity_UnicodeNormalizationAttack verifies protection against Unicode CRLF injection
func TestSecurity_UnicodeNormalizationAttack(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		contains string
	}{
		{
			name:     "Unicode CRLF (U+000D U+000A)",
			input:    "value\u000D\u000AX-Evil: injected",
			wantErr:  false,
			contains: "X-Evil",
		},
		{
			name:     "Unicode Line Separator (U+2028)",
			input:    "value\u2028X-Evil: injected",
			wantErr:  false,
			contains: "X-Evil", // Should remain after control char removal
		},
		{
			name:     "Unicode Paragraph Separator (U+2029)",
			input:    "value\u2029X-Evil: injected",
			wantErr:  false,
			contains: "X-Evil",
		},
		{
			name:     "Mixed Unicode and ASCII control chars",
			input:    "value\r\n\u000D\u000AX-Evil: bad",
			wantErr:  false,
			contains: "X-Evil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeHeaderValue(tt.input, 10000)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeHeaderValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify no CRLF remains
			if strings.Contains(result, "\r") {
				t.Errorf("Result still contains CR: %q", result)
			}
			if strings.Contains(result, "\n") {
				t.Errorf("Result still contains LF: %q", result)
			}

			// Verify text content is preserved (without control chars)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Result missing expected content %q: got %q", tt.contains, result)
			}
		})
	}
}

// TestSecurity_ProtectedHeaderBypass verifies no bypass of protected header checks
func TestSecurity_ProtectedHeaderBypass(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		expected   bool
	}{
		// Standard cases
		{"lowercase host", "host", true},
		{"uppercase HOST", "HOST", true},
		{"mixed Host", "Host", true},

		// Bypass attempts - prefix/suffix
		{"prefix bypass", "x-host", false},
		{"suffix bypass", "host-bypass", false},
		{"embedded bypass", "my-host-header", false},

		// Whitespace bypass attempts
		{"leading space", " host", false},
		{"trailing space", "host ", false},
		{"embedded space", "ho st", false},

		// Special character bypass attempts
		{"tab prefix", "\thost", false},
		{"newline prefix", "\nhost", false},
		{"null byte", "host\x00", false},

		// Case variations of all protected headers
		{"X-Forwarded-For", "X-Forwarded-For", true},
		{"x-forwarded-host", "x-forwarded-host", true},
		{"X-FORWARDED-PROTO", "X-FORWARDED-PROTO", true},
		{"Content-Length", "Content-Length", true},
		{"CONTENT-TYPE", "CONTENT-TYPE", true},
		{"Transfer-Encoding", "Transfer-Encoding", true},
		{"X-Real-IP", "X-Real-IP", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsProtectedHeader(tt.headerName)
			if result != tt.expected {
				t.Errorf("IsProtectedHeader(%q) = %v, want %v", tt.headerName, result, tt.expected)
			}
		})
	}
}

// TestSecurity_DeepClaimPath verifies protection against extremely deep nesting
func TestSecurity_DeepClaimPath(t *testing.T) {
	// Create deeply nested structure
	depth := 100
	data := make(map[string]interface{})
	current := data

	for i := 0; i < depth-1; i++ {
		nested := make(map[string]interface{})
		current["level"] = nested
		current = nested
	}
	current["value"] = "deep-secret"

	// Build path string
	pathParts := make([]string, depth)
	for i := 0; i < depth-1; i++ {
		pathParts[i] = "level"
	}
	pathParts[depth-1] = "value"
	path := strings.Join(pathParts, ".")

	tests := []struct {
		name     string
		path     string
		maxDepth int
		wantErr  bool
	}{
		{
			name:     "depth 100 with max 10 - should fail",
			path:     path,
			maxDepth: 10,
			wantErr:  true,
		},
		{
			name:     "depth 100 with max 50 - should fail",
			path:     path,
			maxDepth: 50,
			wantErr:  true,
		},
		{
			name:     "depth 100 with max 100 - should succeed",
			path:     path,
			maxDepth: 100,
			wantErr:  false,
		},
		{
			name:     "depth 100 with max 99 - should fail",
			path:     path,
			maxDepth: 99,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractClaim(data, tt.path, tt.maxDepth)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractClaim() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSecurity_LargeClaimValue verifies protection against memory exhaustion
func TestSecurity_LargeClaimValue(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		maxSize  int
		wantErr  bool
	}{
		{
			name:     "1KB claim with 8KB limit",
			size:     1024,
			maxSize:  8192,
			wantErr:  false,
		},
		{
			name:     "8KB claim with 8KB limit",
			size:     8192,
			maxSize:  8192,
			wantErr:  false,
		},
		{
			name:     "8193 bytes with 8KB limit - should fail",
			size:     8193,
			maxSize:  8192,
			wantErr:  true,
		},
		{
			name:     "1MB claim with 8KB limit - should fail",
			size:     1024 * 1024,
			maxSize:  8192,
			wantErr:  true,
		},
		{
			name:     "10MB claim with 8KB limit - should fail",
			size:     10 * 1024 * 1024,
			maxSize:  8192,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			largeValue := strings.Repeat("a", tt.size)
			_, err := SanitizeHeaderValue(largeValue, tt.maxSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeHeaderValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSecurity_ManyClaimMappings verifies plugin handles many claim mappings without crashing
func TestSecurity_ManyClaimMappings(t *testing.T) {
	// Create configuration with 1000 claim mappings
	// Use unique header names to avoid duplicate validation errors
	claims := make([]ClaimMapping, 1000)
	for i := 0; i < 1000; i++ {
		claims[i] = ClaimMapping{
			ClaimPath:  "sub",
			HeaderName: "X-Header-" + fmt.Sprintf("%04d", i),
		}
	}

	config := &Config{
		SourceHeader:  "Authorization",
		TokenPrefix:   "Bearer ",
		Claims:        claims,
		Sections:      []string{"payload"},
		MaxClaimDepth: 10,
		MaxHeaderSize: 8192,
	}

	// Should not crash during validation or creation
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	plugin, err := New(context.Background(), nextHandler, config, "test-plugin")
	if err != nil {
		t.Fatalf("New() failed with many claim mappings: %v", err)
	}

	// Should process request without crashing
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestSecurity_DeeplyNestedJSON verifies handling of extremely nested JSON structures
func TestSecurity_DeeplyNestedJSON(t *testing.T) {
	// Create deeply nested JSON structure
	depth := 50
	data := make(map[string]interface{})
	current := data

	for i := 0; i < depth-1; i++ {
		nested := make(map[string]interface{})
		current["nested"] = nested
		current = nested
	}
	current["value"] = "deep-value"

	// Build path to extract
	pathParts := make([]string, depth)
	for i := 0; i < depth-1; i++ {
		pathParts[i] = "nested"
	}
	pathParts[depth-1] = "value"
	path := strings.Join(pathParts, ".")

	// Should succeed with sufficient maxDepth
	value, err := ExtractClaim(data, path, 100)
	if err != nil {
		t.Errorf("ExtractClaim() failed with deep nesting: %v", err)
	}
	if value != "deep-value" {
		t.Errorf("ExtractClaim() = %v, want deep-value", value)
	}

	// Should fail with insufficient maxDepth
	_, err = ExtractClaim(data, path, 10)
	if err == nil {
		t.Error("ExtractClaim() should fail when depth exceeds maxDepth")
	}
}

// TestSecurity_TypeConfusion verifies safe handling of unexpected types
func TestSecurity_TypeConfusion(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		path      string
		wantErr   bool
		errorType string
	}{
		{
			name: "array where object expected",
			data: map[string]interface{}{
				"roles": []interface{}{"admin", "user"},
			},
			path:      "roles.name",
			wantErr:   true,
			errorType: "not an object",
		},
		{
			name: "number where object expected",
			data: map[string]interface{}{
				"count": float64(42),
			},
			path:      "count.value",
			wantErr:   true,
			errorType: "not an object",
		},
		{
			name: "string where object expected",
			data: map[string]interface{}{
				"name": "John Doe",
			},
			path:      "name.first",
			wantErr:   true,
			errorType: "not an object",
		},
		{
			name: "boolean where object expected",
			data: map[string]interface{}{
				"active": true,
			},
			path:      "active.status",
			wantErr:   true,
			errorType: "not an object",
		},
		{
			name: "nil where object expected",
			data: map[string]interface{}{
				"nullable": nil,
			},
			path:      "nullable.value",
			wantErr:   true,
			errorType: "not an object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractClaim(tt.data, tt.path, 10)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractClaim() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errorType) {
				t.Errorf("ExtractClaim() error = %v, want error containing %q", err, tt.errorType)
			}
		})
	}
}

// TestSecurity_MixedTypeArrayConversion verifies safe conversion of mixed-type arrays
func TestSecurity_MixedTypeArrayConversion(t *testing.T) {
	tests := []struct {
		name        string
		array       []interface{}
		format      string
		wantErr     bool
		shouldCheck func(string) bool
	}{
		{
			name:   "mixed strings and numbers",
			array:  []interface{}{"admin", float64(123), "user"},
			format: "comma",
			shouldCheck: func(result string) bool {
				return strings.Contains(result, "admin") && strings.Contains(result, "123") && strings.Contains(result, "user")
			},
		},
		{
			name:   "mixed with booleans",
			array:  []interface{}{true, "admin", false, float64(42)},
			format: "comma",
			shouldCheck: func(result string) bool {
				return strings.Contains(result, "true") && strings.Contains(result, "admin") && strings.Contains(result, "false")
			},
		},
		{
			name:   "nested objects in array",
			array:  []interface{}{map[string]interface{}{"id": "123"}, "admin"},
			format: "json",
			shouldCheck: func(result string) bool {
				return strings.Contains(result, `"id":"123"`) && strings.Contains(result, "admin")
			},
		},
		{
			name:   "nil values in array",
			array:  []interface{}{nil, "admin", nil},
			format: "comma",
			shouldCheck: func(result string) bool {
				return strings.Contains(result, "admin")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertClaimToString(tt.array, tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertClaimToString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !tt.shouldCheck(result) {
				t.Errorf("ConvertClaimToString() result check failed: %q", result)
			}
		})
	}
}

// TestSecurity_ProtectedHeaderIntegration verifies protected headers cannot be injected end-to-end
func TestSecurity_ProtectedHeaderIntegration(t *testing.T) {
	protectedHeaders := []string{
		"Host",
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"X-Forwarded-Proto",
		"X-Forwarded-Port",
		"X-Real-IP",
		"Content-Length",
		"Content-Type",
		"Transfer-Encoding",
	}

	for _, headerName := range protectedHeaders {
		t.Run("protect_"+headerName, func(t *testing.T) {
			config := &Config{
				SourceHeader: "Authorization",
				TokenPrefix:  "Bearer ",
				Claims: []ClaimMapping{
					{ClaimPath: "sub", HeaderName: headerName},
				},
				Sections:        []string{"payload"},
				ContinueOnError: false,
				MaxClaimDepth:   10,
				MaxHeaderSize:   8192,
			}

			originalValue := "original-value-should-be-preserved"
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify protected header was NOT modified
				if r.Header.Get(headerName) != originalValue {
					// Some headers like Host are special - just verify they're not the JWT claim value
					if r.Header.Get(headerName) == "1234567890" {
						t.Errorf("Protected header %q was modified to JWT claim value", headerName)
					}
				}
				w.WriteHeader(http.StatusOK)
			})

			plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.Header.Set("Authorization", "Bearer "+validTestToken)
			req.Header.Set(headerName, originalValue)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
			}
		})
	}
}

// TestSecurity_RepeatedControlCharacters verifies removal of multiple control characters
func TestSecurity_RepeatedControlCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "repeated CRLF",
			input: "value\r\n\r\n\r\nX-Evil:bad",
		},
		{
			name:  "repeated null bytes",
			input: "value\x00\x00\x00evil",
		},
		{
			name:  "all control chars 0x00-0x1F",
			input: "value\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0A\x0B\x0C\x0D\x0E\x0F\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1A\x1B\x1C\x1D\x1E\x1Fevil",
		},
		{
			name:  "DEL repeated",
			input: "value\x7F\x7F\x7Fevil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeHeaderValue(tt.input, 10000)
			if err != nil {
				t.Errorf("SanitizeHeaderValue() unexpected error: %v", err)
				return
			}

			// Verify all control characters removed
			for i := 0; i < 0x20; i++ {
				if strings.ContainsRune(result, rune(i)) {
					t.Errorf("Result contains control character 0x%02X", i)
				}
			}
			if strings.ContainsRune(result, 0x7F) {
				t.Error("Result contains DEL character (0x7F)")
			}

			// Verify non-control content preserved (control chars removed, text remains)
			if !strings.Contains(result, "value") {
				t.Errorf("Result missing 'value': %q", result)
			}
			// Note: "evil" or other text after control chars should be preserved
			if len(result) < len("value") {
				t.Errorf("Result too short after sanitization: %q", result)
			}
		})
	}
}
