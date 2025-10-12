package traefik_jwt_decoder_plugin

import (
	"testing"
)

// TestExtractClaim_Simple verifies extraction of top-level claims
func TestExtractClaim_Simple(t *testing.T) {
	data := map[string]interface{}{
		"sub":   "1234567890",
		"email": "test@example.com",
		"iat":   float64(1516239022),
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
	}{
		{
			name:     "string claim",
			path:     "sub",
			expected: "1234567890",
		},
		{
			name:     "email claim",
			path:     "email",
			expected: "test@example.com",
		},
		{
			name:     "numeric claim",
			path:     "iat",
			expected: float64(1516239022),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := ExtractClaim(data, tt.path, 10)
			if err != nil {
				t.Fatalf("ExtractClaim() error: %v", err)
			}
			if value != tt.expected {
				t.Errorf("ExtractClaim() = %v, want %v", value, tt.expected)
			}
		})
	}
}

// TestExtractClaim_Nested verifies extraction of nested claims with dot notation
func TestExtractClaim_Nested(t *testing.T) {
	data := map[string]interface{}{
		"custom": map[string]interface{}{
			"tenant_id": "tenant-123",
			"region":    "us-west",
		},
		"user": map[string]interface{}{
			"profile": map[string]interface{}{
				"name": "John Doe",
				"age":  float64(30),
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
	}{
		{
			name:     "two-level nested",
			path:     "custom.tenant_id",
			expected: "tenant-123",
		},
		{
			name:     "two-level nested region",
			path:     "custom.region",
			expected: "us-west",
		},
		{
			name:     "three-level nested",
			path:     "user.profile.name",
			expected: "John Doe",
		},
		{
			name:     "three-level nested numeric",
			path:     "user.profile.age",
			expected: float64(30),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := ExtractClaim(data, tt.path, 10)
			if err != nil {
				t.Fatalf("ExtractClaim() error: %v", err)
			}
			if value != tt.expected {
				t.Errorf("ExtractClaim() = %v, want %v", value, tt.expected)
			}
		})
	}
}

// TestExtractClaim_DeepNested verifies extraction of deeply nested claims
func TestExtractClaim_DeepNested(t *testing.T) {
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": map[string]interface{}{
					"d": map[string]interface{}{
						"e": "deep-value",
					},
				},
			},
		},
	}

	value, err := ExtractClaim(data, "a.b.c.d.e", 10)
	if err != nil {
		t.Fatalf("ExtractClaim() error: %v", err)
	}
	if value != "deep-value" {
		t.Errorf("ExtractClaim() = %v, want deep-value", value)
	}
}

// TestExtractClaim_NotFound verifies error handling for missing claims
func TestExtractClaim_NotFound(t *testing.T) {
	data := map[string]interface{}{
		"sub":   "1234567890",
		"email": "test@example.com",
	}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "missing top-level claim",
			path: "missing",
		},
		{
			name: "missing nested claim",
			path: "custom.tenant_id",
		},
		{
			name: "partial path exists",
			path: "sub.nested",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := ExtractClaim(data, tt.path, 10)
			if err == nil {
				t.Errorf("ExtractClaim() expected error for missing path, got value: %v", value)
			}
			if value != nil {
				t.Errorf("ExtractClaim() expected nil value, got: %v", value)
			}
		})
	}
}

// TestExtractClaim_DepthExceeded verifies error handling when path exceeds max depth
func TestExtractClaim_DepthExceeded(t *testing.T) {
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "value",
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		maxDepth int
		wantErr  bool
	}{
		{
			name:     "depth 3 with max 3",
			path:     "a.b.c",
			maxDepth: 3,
			wantErr:  false,
		},
		{
			name:     "depth 3 with max 2",
			path:     "a.b.c",
			maxDepth: 2,
			wantErr:  true,
		},
		{
			name:     "depth 1 with max 1",
			path:     "a",
			maxDepth: 1,
			wantErr:  false,
		},
		{
			name:     "depth exceeds by 1",
			path:     "a.b.c",
			maxDepth: 2,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractClaim(data, tt.path, tt.maxDepth)
			if tt.wantErr && err == nil {
				t.Errorf("ExtractClaim() expected error for depth exceeded, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ExtractClaim() unexpected error: %v", err)
			}
		})
	}
}

// TestExtractClaim_InvalidPath verifies error when path segment is not an object
func TestExtractClaim_InvalidPath(t *testing.T) {
	data := map[string]interface{}{
		"roles": []interface{}{"admin", "user"},
		"count": float64(42),
	}

	tests := []struct {
		name string
		path string
	}{
		{
			name: "array is not object",
			path: "roles.name",
		},
		{
			name: "number is not object",
			path: "count.value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := ExtractClaim(data, tt.path, 10)
			if err == nil {
				t.Errorf("ExtractClaim() expected error for invalid path, got value: %v", value)
			}
		})
	}
}

// TestExtractClaim_NilValue verifies handling of nil values in claims
func TestExtractClaim_NilValue(t *testing.T) {
	data := map[string]interface{}{
		"nullable": nil,
	}

	value, err := ExtractClaim(data, "nullable", 10)
	if err != nil {
		t.Fatalf("ExtractClaim() error: %v", err)
	}
	if value != nil {
		t.Errorf("ExtractClaim() = %v, want nil", value)
	}
}

// TestConvertClaimToString_String verifies string conversion
func TestConvertClaimToString_String(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "simple string",
			value:    "hello",
			expected: "hello",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "",
		},
		{
			name:     "string with spaces",
			value:    "hello world",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertClaimToString(tt.value, "")
			if err != nil {
				t.Fatalf("ConvertClaimToString() error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("ConvertClaimToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestConvertClaimToString_Int verifies integer conversion
func TestConvertClaimToString_Int(t *testing.T) {
	result, err := ConvertClaimToString(42, "")
	if err != nil {
		t.Fatalf("ConvertClaimToString() error: %v", err)
	}
	if result != "42" {
		t.Errorf("ConvertClaimToString() = %q, want 42", result)
	}
}

// TestConvertClaimToString_Float verifies float conversion
func TestConvertClaimToString_Float(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{
			name:     "integer float",
			value:    42.0,
			expected: "42",
		},
		{
			name:     "decimal float",
			value:    3.14159,
			expected: "3.14159",
		},
		{
			name:     "negative float",
			value:    -123.456,
			expected: "-123.456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertClaimToString(tt.value, "")
			if err != nil {
				t.Fatalf("ConvertClaimToString() error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("ConvertClaimToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestConvertClaimToString_Bool verifies boolean conversion
func TestConvertClaimToString_Bool(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		expected string
	}{
		{
			name:     "true",
			value:    true,
			expected: "true",
		},
		{
			name:     "false",
			value:    false,
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertClaimToString(tt.value, "")
			if err != nil {
				t.Fatalf("ConvertClaimToString() error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("ConvertClaimToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestConvertClaimToString_Nil verifies nil conversion
func TestConvertClaimToString_Nil(t *testing.T) {
	result, err := ConvertClaimToString(nil, "")
	if err != nil {
		t.Fatalf("ConvertClaimToString() error: %v", err)
	}
	if result != "" {
		t.Errorf("ConvertClaimToString() = %q, want empty string", result)
	}
}

// TestConvertClaimToString_Array_Comma verifies array to comma-separated string
func TestConvertClaimToString_Array_Comma(t *testing.T) {
	tests := []struct {
		name     string
		value    []interface{}
		format   string
		expected string
	}{
		{
			name:     "string array default format",
			value:    []interface{}{"admin", "user"},
			format:   "",
			expected: "admin, user",
		},
		{
			name:     "string array comma format",
			value:    []interface{}{"admin", "user"},
			format:   "comma",
			expected: "admin, user",
		},
		{
			name:     "mixed type array",
			value:    []interface{}{"admin", float64(123), true},
			format:   "comma",
			expected: "admin, 123, true",
		},
		{
			name:     "single element",
			value:    []interface{}{"admin"},
			format:   "comma",
			expected: "admin",
		},
		{
			name:     "empty array",
			value:    []interface{}{},
			format:   "comma",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertClaimToString(tt.value, tt.format)
			if err != nil {
				t.Fatalf("ConvertClaimToString() error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("ConvertClaimToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestConvertClaimToString_Array_JSON verifies array to JSON string
func TestConvertClaimToString_Array_JSON(t *testing.T) {
	tests := []struct {
		name     string
		value    []interface{}
		expected string
	}{
		{
			name:     "string array",
			value:    []interface{}{"admin", "user"},
			expected: `["admin","user"]`,
		},
		{
			name:     "mixed type array",
			value:    []interface{}{"admin", float64(123), true},
			expected: `["admin",123,true]`,
		},
		{
			name:     "single element",
			value:    []interface{}{"admin"},
			expected: `["admin"]`,
		},
		{
			name:     "empty array",
			value:    []interface{}{},
			expected: `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertClaimToString(tt.value, "json")
			if err != nil {
				t.Fatalf("ConvertClaimToString() error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("ConvertClaimToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestConvertClaimToString_Object verifies object to JSON string
func TestConvertClaimToString_Object(t *testing.T) {
	tests := []struct {
		name     string
		value    map[string]interface{}
		expected string
	}{
		{
			name: "simple object",
			value: map[string]interface{}{
				"key": "value",
			},
			expected: `{"key":"value"}`,
		},
		{
			name: "nested object",
			value: map[string]interface{}{
				"user": map[string]interface{}{
					"id": "123",
				},
			},
			expected: `{"user":{"id":"123"}}`,
		},
		{
			name:     "empty object",
			value:    map[string]interface{}{},
			expected: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertClaimToString(tt.value, "")
			if err != nil {
				t.Fatalf("ConvertClaimToString() error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("ConvertClaimToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}
