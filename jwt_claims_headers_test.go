package traefik_jwt_decoder_plugin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// validTestToken is the standard test JWT from CLAUDE.md
const validTestToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwicm9sZXMiOlsiYWRtaW4iLCJ1c2VyIl0sImN1c3RvbSI6eyJ0ZW5hbnRfaWQiOiJ0ZW5hbnQtMTIzIn0sImlhdCI6MTUxNjIzOTAyMn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

// TestNew_ValidConfig verifies plugin creation with valid configuration
func TestNew_ValidConfig(t *testing.T) {
	config := &Config{
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:      []string{"payload"},
		MaxClaimDepth: 10,
		MaxHeaderSize: 8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler, err := New(context.Background(), nextHandler, config, "test-plugin")
	if err != nil {
		t.Fatalf("New() failed with valid config: %v", err)
	}
	if handler == nil {
		t.Fatal("New() returned nil handler")
	}
}

// TestNew_InvalidConfig verifies plugin creation rejects invalid configuration
func TestNew_InvalidConfig(t *testing.T) {
	config := &Config{
		Claims:        []ClaimMapping{}, // Empty claims - invalid
		Sections:      []string{"payload"},
		MaxClaimDepth: 10,
		MaxHeaderSize: 8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler, err := New(context.Background(), nextHandler, config, "test-plugin")
	if err == nil {
		t.Fatal("New() expected error for invalid config, got nil")
	}
	if handler != nil {
		t.Fatalf("New() expected nil handler for invalid config, got %v", handler)
	}
}

// TestServeHTTP_ValidJWT verifies successful JWT processing with valid token
func TestServeHTTP_ValidJWT(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
			{ClaimPath: "email", HeaderName: "X-User-Email"},
			{ClaimPath: "custom.tenant_id", HeaderName: "X-Tenant-Id"},
		},
		Sections:           []string{"payload"},
		ContinueOnError:    false,
		RemoveSourceHeader: false,
		MaxClaimDepth:      10,
		MaxHeaderSize:      8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify injected headers
		if r.Header.Get("X-User-Id") != "1234567890" {
			t.Errorf("X-User-Id = %q, want 1234567890", r.Header.Get("X-User-Id"))
		}
		if r.Header.Get("X-User-Email") != "test@example.com" {
			t.Errorf("X-User-Email = %q, want test@example.com", r.Header.Get("X-User-Email"))
		}
		if r.Header.Get("X-Tenant-Id") != "tenant-123" {
			t.Errorf("X-Tenant-Id = %q, want tenant-123", r.Header.Get("X-Tenant-Id"))
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_MissingJWT_ContinueOnError verifies pass-through when JWT missing with continueOnError=true
func TestServeHTTP_MissingJWT_ContinueOnError(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:        []string{"payload"},
		ContinueOnError: true,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	// No Authorization header

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("Next handler should be called when continueOnError=true")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_MissingJWT_StrictMode verifies 401 error when JWT missing with continueOnError=false
func TestServeHTTP_MissingJWT_StrictMode(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:        []string{"payload"},
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	// No Authorization header

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if nextCalled {
		t.Error("Next handler should NOT be called when continueOnError=false")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	// Verify JSON error response
	body, _ := io.ReadAll(rr.Body)
	var errorResponse map[string]string
	if err := json.Unmarshal(body, &errorResponse); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}
	if errorResponse["error"] != "unauthorized" {
		t.Errorf("Error type = %q, want unauthorized", errorResponse["error"])
	}
}

// TestServeHTTP_MalformedJWT_ContinueOnError verifies pass-through with malformed JWT and continueOnError=true
func TestServeHTTP_MalformedJWT_ContinueOnError(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:        []string{"payload"},
		ContinueOnError: true,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer invalid.token")

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("Next handler should be called when continueOnError=true")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_MalformedJWT_StrictMode verifies 401 error with malformed JWT and continueOnError=false
func TestServeHTTP_MalformedJWT_StrictMode(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:        []string{"payload"},
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer invalid.token")

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if nextCalled {
		t.Error("Next handler should NOT be called when continueOnError=false")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// TestServeHTTP_MultipleClaimMappings verifies multiple claim extractions
func TestServeHTTP_MultipleClaimMappings(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
			{ClaimPath: "email", HeaderName: "X-User-Email"},
			{ClaimPath: "custom.tenant_id", HeaderName: "X-Tenant-Id"},
			{ClaimPath: "roles", HeaderName: "X-Roles", ArrayFormat: "comma"},
		},
		Sections:        []string{"payload"},
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify all headers injected
		if r.Header.Get("X-User-Id") != "1234567890" {
			t.Errorf("X-User-Id = %q, want 1234567890", r.Header.Get("X-User-Id"))
		}
		if r.Header.Get("X-User-Email") != "test@example.com" {
			t.Errorf("X-User-Email = %q, want test@example.com", r.Header.Get("X-User-Email"))
		}
		if r.Header.Get("X-Tenant-Id") != "tenant-123" {
			t.Errorf("X-Tenant-Id = %q, want tenant-123", r.Header.Get("X-Tenant-Id"))
		}
		if r.Header.Get("X-Roles") != "admin, user" {
			t.Errorf("X-Roles = %q, want 'admin, user'", r.Header.Get("X-Roles"))
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_HeaderSection verifies reading from JWT header section
func TestServeHTTP_HeaderSection(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "alg", HeaderName: "X-JWT-Alg"},
			{ClaimPath: "typ", HeaderName: "X-JWT-Type"},
		},
		Sections:        []string{"header"},
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-JWT-Alg") != "HS256" {
			t.Errorf("X-JWT-Alg = %q, want HS256", r.Header.Get("X-JWT-Alg"))
		}
		if r.Header.Get("X-JWT-Type") != "JWT" {
			t.Errorf("X-JWT-Type = %q, want JWT", r.Header.Get("X-JWT-Type"))
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_PayloadSection verifies reading from JWT payload section
func TestServeHTTP_PayloadSection(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:        []string{"payload"},
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-Id") != "1234567890" {
			t.Errorf("X-User-Id = %q, want 1234567890", r.Header.Get("X-User-Id"))
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_FallbackSection verifies fallback from payload to header
func TestServeHTTP_FallbackSection(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "alg", HeaderName: "X-JWT-Alg"}, // Only in header
			{ClaimPath: "sub", HeaderName: "X-User-Id"}, // Only in payload
		},
		Sections:        []string{"payload", "header"}, // Try payload first, fallback to header
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-JWT-Alg") != "HS256" {
			t.Errorf("X-JWT-Alg = %q, want HS256", r.Header.Get("X-JWT-Alg"))
		}
		if r.Header.Get("X-User-Id") != "1234567890" {
			t.Errorf("X-User-Id = %q, want 1234567890", r.Header.Get("X-User-Id"))
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_RemoveSourceHeader verifies source header removal
func TestServeHTTP_RemoveSourceHeader(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:           []string{"payload"},
		ContinueOnError:    false,
		RemoveSourceHeader: true,
		MaxClaimDepth:      10,
		MaxHeaderSize:      8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header removed
		if r.Header.Get("Authorization") != "" {
			t.Errorf("Authorization header should be removed, got: %q", r.Header.Get("Authorization"))
		}
		// Verify claim still injected
		if r.Header.Get("X-User-Id") != "1234567890" {
			t.Errorf("X-User-Id = %q, want 1234567890", r.Header.Get("X-User-Id"))
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_OverrideExistingHeader verifies override=true behavior
func TestServeHTTP_OverrideExistingHeader(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id", Override: true},
		},
		Sections:        []string{"payload"},
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should be overridden with JWT claim value
		if r.Header.Get("X-User-Id") != "1234567890" {
			t.Errorf("X-User-Id = %q, want 1234567890 (should be overridden)", r.Header.Get("X-User-Id"))
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)
	req.Header.Set("X-User-Id", "existing-value")

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_PreserveExistingHeader verifies override=false behavior
func TestServeHTTP_PreserveExistingHeader(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id", Override: false},
		},
		Sections:        []string{"payload"},
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should preserve existing value
		if r.Header.Get("X-User-Id") != "existing-value" {
			t.Errorf("X-User-Id = %q, want existing-value (should be preserved)", r.Header.Get("X-User-Id"))
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)
	req.Header.Set("X-User-Id", "existing-value")

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_ArrayFormatComma verifies comma-separated array formatting
func TestServeHTTP_ArrayFormatComma(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "roles", HeaderName: "X-Roles", ArrayFormat: "comma"},
		},
		Sections:        []string{"payload"},
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Roles") != "admin, user" {
			t.Errorf("X-Roles = %q, want 'admin, user'", r.Header.Get("X-Roles"))
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServeHTTP_ArrayFormatJSON verifies JSON array formatting
func TestServeHTTP_ArrayFormatJSON(t *testing.T) {
	config := &Config{
		SourceHeader: "Authorization",
		TokenPrefix:  "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "roles", HeaderName: "X-Roles", ArrayFormat: "json"},
		},
		Sections:        []string{"payload"},
		ContinueOnError: false,
		MaxClaimDepth:   10,
		MaxHeaderSize:   8192,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := `["admin","user"]`
		if r.Header.Get("X-Roles") != expected {
			t.Errorf("X-Roles = %q, want %q", r.Header.Get("X-Roles"), expected)
		}
		w.WriteHeader(http.StatusOK)
	})

	plugin, _ := New(context.Background(), nextHandler, config, "test-plugin")

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Authorization", "Bearer "+validTestToken)

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rr.Code, http.StatusOK)
	}
}
