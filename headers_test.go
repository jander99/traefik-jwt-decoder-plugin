package traefik_jwt_decoder_plugin

import (
	"net/http"
	"strings"
	"testing"
)

// TestIsProtectedHeader_CaseInsensitive verifies case-insensitive protection
func TestIsProtectedHeader_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected bool
	}{
		{"lowercase host", "host", true},
		{"uppercase HOST", "HOST", true},
		{"mixed case Host", "Host", true},
		{"x-forwarded-for lowercase", "x-forwarded-for", true},
		{"x-forwarded-for mixed", "X-Forwarded-For", true},
		{"x-forwarded-for uppercase", "X-FORWARDED-FOR", true},
		{"content-length", "Content-Length", true},
		{"safe header", "X-Custom-Header", false},
		{"safe lowercase", "x-user-id", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsProtectedHeader(tt.header)
			if result != tt.expected {
				t.Errorf("IsProtectedHeader(%q) = %v, want %v", tt.header, result, tt.expected)
			}
		})
	}
}

// TestSanitizeHeaderValue_ControlChars verifies control character removal
func TestSanitizeHeaderValue_ControlChars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxSize  int
		expected string
		wantErr  bool
	}{
		{
			name:     "clean value",
			input:    "normal-value-123",
			maxSize:  1000,
			expected: "normal-value-123",
			wantErr:  false,
		},
		{
			name:     "null byte",
			input:    "test\x00value",
			maxSize:  1000,
			expected: "testvalue",
			wantErr:  false,
		},
		{
			name:     "multiple control chars",
			input:    "test\x01\x02\x03value",
			maxSize:  1000,
			expected: "testvalue",
			wantErr:  false,
		},
		{
			name:     "DEL character (0x7F)",
			input:    "test\x7Fvalue",
			maxSize:  1000,
			expected: "testvalue",
			wantErr:  false,
		},
		{
			name:     "whitespace trimming",
			input:    "  value with spaces  ",
			maxSize:  1000,
			expected: "value with spaces",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeHeaderValue(tt.input, tt.maxSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeHeaderValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("SanitizeHeaderValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestSanitizeHeaderValue_HeaderInjection verifies header injection prevention
func TestSanitizeHeaderValue_HeaderInjection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxSize  int
		expected string
		wantErr  bool
	}{
		{
			name:     "CRLF injection attempt",
			input:    "value\r\nX-Evil: injected\r\n",
			maxSize:  1000,
			expected: "valueX-Evil: injected",
			wantErr:  false,
		},
		{
			name:     "LF injection attempt",
			input:    "value\nX-Evil: injected",
			maxSize:  1000,
			expected: "valueX-Evil: injected",
			wantErr:  false,
		},
		{
			name:     "CR injection attempt",
			input:    "value\rX-Evil: injected",
			maxSize:  1000,
			expected: "valueX-Evil: injected",
			wantErr:  false,
		},
		{
			name:     "mixed control chars in injection",
			input:    "value\r\n\x00X-Evil: bad\x01\x02",
			maxSize:  1000,
			expected: "valueX-Evil: bad",
			wantErr:  false,
		},
		{
			name:     "size limit exceeded",
			input:    strings.Repeat("a", 10001),
			maxSize:  10000,
			expected: "",
			wantErr:  true,
		},
		{
			name:     "exact size limit",
			input:    strings.Repeat("a", 10000),
			maxSize:  10000,
			expected: strings.Repeat("a", 10000),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeHeaderValue(tt.input, tt.maxSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeHeaderValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("SanitizeHeaderValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestInjectHeader_Protected verifies protected header blocking
func TestInjectHeader_Protected(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		value      string
		override   bool
		shouldSkip bool
	}{
		{"protected host", "Host", "evil.com", true, true},
		{"protected HOST uppercase", "HOST", "evil.com", true, true},
		{"protected x-forwarded-for", "X-Forwarded-For", "1.2.3.4", true, true},
		{"protected content-length", "Content-Length", "999", true, true},
		{"safe custom header", "X-User-ID", "12345", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)

			err := InjectHeader(req, tt.headerName, tt.value, tt.override, 10000)
			if err != nil {
				t.Errorf("InjectHeader() unexpected error: %v", err)
			}

			// Check if header was injected
			injected := req.Header.Get(tt.headerName)
			if tt.shouldSkip {
				if injected != "" {
					t.Errorf("Protected header %q was injected with value %q", tt.headerName, injected)
				}
			} else {
				if injected == "" {
					t.Errorf("Safe header %q was not injected", tt.headerName)
				}
			}
		})
	}
}

// TestInjectHeader_OverrideBehavior verifies override flag behavior
func TestInjectHeader_OverrideBehavior(t *testing.T) {
	tests := []struct {
		name           string
		existingValue  string
		newValue       string
		override       bool
		expectedValue  string
		shouldOverride bool
	}{
		{
			name:           "override true - should replace",
			existingValue:  "old-value",
			newValue:       "new-value",
			override:       true,
			expectedValue:  "new-value",
			shouldOverride: true,
		},
		{
			name:           "override false - should keep existing",
			existingValue:  "old-value",
			newValue:       "new-value",
			override:       false,
			expectedValue:  "old-value",
			shouldOverride: false,
		},
		{
			name:           "no existing header - should inject",
			existingValue:  "",
			newValue:       "new-value",
			override:       false,
			expectedValue:  "new-value",
			shouldOverride: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)

			// Set existing header if provided
			if tt.existingValue != "" {
				req.Header.Set("X-Test-Header", tt.existingValue)
			}

			err := InjectHeader(req, "X-Test-Header", tt.newValue, tt.override, 10000)
			if err != nil {
				t.Errorf("InjectHeader() unexpected error: %v", err)
			}

			result := req.Header.Get("X-Test-Header")
			if result != tt.expectedValue {
				t.Errorf("Header value = %q, want %q", result, tt.expectedValue)
			}
		})
	}
}

// TestInjectHeader_Sanitization verifies sanitization during injection
func TestInjectHeader_Sanitization(t *testing.T) {
	tests := []struct {
		name          string
		headerName    string
		inputValue    string
		expectedValue string
		wantErr       bool
	}{
		{
			name:          "clean value",
			headerName:    "X-User-ID",
			inputValue:    "user-123",
			expectedValue: "user-123",
			wantErr:       false,
		},
		{
			name:          "value with control chars",
			headerName:    "X-User-Name",
			inputValue:    "John\x00Doe",
			expectedValue: "JohnDoe",
			wantErr:       false,
		},
		{
			name:          "value with CRLF",
			headerName:    "X-Data",
			inputValue:    "value\r\nX-Evil: bad",
			expectedValue: "valueX-Evil: bad",
			wantErr:       false,
		},
		{
			name:          "value exceeds size",
			headerName:    "X-Large",
			inputValue:    strings.Repeat("a", 101),
			expectedValue: "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)

			err := InjectHeader(req, tt.headerName, tt.inputValue, true, 100)
			if (err != nil) != tt.wantErr {
				t.Errorf("InjectHeader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				result := req.Header.Get(tt.headerName)
				if result != tt.expectedValue {
					t.Errorf("Header value = %q, want %q", result, tt.expectedValue)
				}
			}
		})
	}
}

// TestInjectHeader_Integration verifies complete injection workflow
func TestInjectHeader_Integration(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	// Test 1: Inject safe header
	err := InjectHeader(req, "X-User-ID", "12345", true, 10000)
	if err != nil {
		t.Fatalf("Failed to inject safe header: %v", err)
	}
	if req.Header.Get("X-User-ID") != "12345" {
		t.Error("Safe header not injected correctly")
	}

	// Test 2: Attempt to inject protected header (should be silently skipped)
	err = InjectHeader(req, "Host", "evil.com", true, 10000)
	if err != nil {
		t.Fatalf("Protected header injection should not error: %v", err)
	}
	// Host header should remain empty or unchanged
	if req.Host == "evil.com" {
		t.Error("Protected header was modified")
	}

	// Test 3: Inject header with dangerous content (should be sanitized)
	err = InjectHeader(req, "X-Custom", "value\r\nX-Evil: bad\r\n", true, 10000)
	if err != nil {
		t.Fatalf("Failed to inject header with sanitization: %v", err)
	}
	sanitized := req.Header.Get("X-Custom")
	if strings.Contains(sanitized, "\r") || strings.Contains(sanitized, "\n") {
		t.Errorf("Header value still contains control characters: %q", sanitized)
	}

	// Test 4: Test override behavior
	req.Header.Set("X-Existing", "original")
	err = InjectHeader(req, "X-Existing", "new-value", false, 10000)
	if err != nil {
		t.Fatalf("Override false should not error: %v", err)
	}
	if req.Header.Get("X-Existing") != "original" {
		t.Error("Header was overridden when override=false")
	}

	// Test 5: Test override=true
	err = InjectHeader(req, "X-Existing", "new-value", true, 10000)
	if err != nil {
		t.Fatalf("Override true should not error: %v", err)
	}
	if req.Header.Get("X-Existing") != "new-value" {
		t.Error("Header was not overridden when override=true")
	}
}
