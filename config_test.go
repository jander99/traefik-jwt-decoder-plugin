package traefik_jwt_decoder_plugin

import (
	"testing"
)

// TestValidate_Valid verifies successful validation of valid configuration
func TestValidate_Valid(t *testing.T) {
	config := &Config{
		SourceHeader:       "Authorization",
		TokenPrefix:        "Bearer ",
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
			{ClaimPath: "email", HeaderName: "X-User-Email"},
		},
		Sections:           []string{"payload"},
		ContinueOnError:    true,
		RemoveSourceHeader: false,
		MaxClaimDepth:      10,
		MaxHeaderSize:      8192,
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Validate() failed for valid config: %v", err)
	}
}

// TestValidate_EmptyClaims verifies error for empty claims array
func TestValidate_EmptyClaims(t *testing.T) {
	config := &Config{
		Claims:        []ClaimMapping{},
		Sections:      []string{"payload"},
		MaxClaimDepth: 10,
		MaxHeaderSize: 8192,
	}

	err := config.Validate()
	if err == nil {
		t.Errorf("Validate() expected error for empty claims, got nil")
	}
}

// TestValidate_EmptyClaimPath verifies error for empty claimPath
func TestValidate_EmptyClaimPath(t *testing.T) {
	config := &Config{
		Claims: []ClaimMapping{
			{ClaimPath: "", HeaderName: "X-User-Id"},
		},
		Sections:      []string{"payload"},
		MaxClaimDepth: 10,
		MaxHeaderSize: 8192,
	}

	err := config.Validate()
	if err == nil {
		t.Errorf("Validate() expected error for empty claimPath, got nil")
	}
}

// TestValidate_EmptyHeaderName verifies error for empty headerName
func TestValidate_EmptyHeaderName(t *testing.T) {
	config := &Config{
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: ""},
		},
		Sections:      []string{"payload"},
		MaxClaimDepth: 10,
		MaxHeaderSize: 8192,
	}

	err := config.Validate()
	if err == nil {
		t.Errorf("Validate() expected error for empty headerName, got nil")
	}
}

// TestValidate_InvalidArrayFormat verifies error for invalid arrayFormat
func TestValidate_InvalidArrayFormat(t *testing.T) {
	tests := []struct {
		name        string
		arrayFormat string
		shouldError bool
	}{
		{
			name:        "invalid arrayFormat",
			arrayFormat: "invalid",
			shouldError: true,
		},
		{
			name:        "uppercase COMMA",
			arrayFormat: "COMMA",
			shouldError: true,
		},
		{
			name:        "comma valid",
			arrayFormat: "comma",
			shouldError: false,
		},
		{
			name:        "json valid",
			arrayFormat: "json",
			shouldError: false,
		},
		{
			name:        "empty string valid",
			arrayFormat: "",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Claims: []ClaimMapping{
					{ClaimPath: "roles", HeaderName: "X-Roles", ArrayFormat: tt.arrayFormat},
				},
				Sections:      []string{"payload"},
				MaxClaimDepth: 10,
				MaxHeaderSize: 8192,
			}

			err := config.Validate()
			if tt.shouldError && err == nil {
				t.Errorf("Validate() expected error for arrayFormat=%q, got nil", tt.arrayFormat)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Validate() unexpected error for arrayFormat=%q: %v", tt.arrayFormat, err)
			}
		})
	}
}

// TestValidate_DuplicateHeaders verifies case-insensitive duplicate header detection
func TestValidate_DuplicateHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		wantErr bool
	}{
		{
			name:    "exact duplicates",
			headers: []string{"X-User-Id", "X-User-Id"},
			wantErr: true,
		},
		{
			name:    "case insensitive duplicates",
			headers: []string{"X-User-Id", "x-user-id"},
			wantErr: true,
		},
		{
			name:    "mixed case duplicates",
			headers: []string{"X-User-Id", "X-USER-ID", "x-User-Id"},
			wantErr: true,
		},
		{
			name:    "unique headers",
			headers: []string{"X-User-Id", "X-User-Email", "X-Tenant-Id"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := make([]ClaimMapping, len(tt.headers))
			for i, header := range tt.headers {
				claims[i] = ClaimMapping{
					ClaimPath:  "sub",
					HeaderName: header,
				}
			}

			config := &Config{
				Claims:        claims,
				Sections:      []string{"payload"},
				MaxClaimDepth: 10,
				MaxHeaderSize: 8192,
			}

			err := config.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() expected error for duplicate headers, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

// TestValidate_EmptySections verifies error for empty sections array
func TestValidate_EmptySections(t *testing.T) {
	config := &Config{
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:      []string{},
		MaxClaimDepth: 10,
		MaxHeaderSize: 8192,
	}

	err := config.Validate()
	if err == nil {
		t.Errorf("Validate() expected error for empty sections, got nil")
	}
}

// TestValidate_InvalidSection verifies error for invalid section names
func TestValidate_InvalidSection(t *testing.T) {
	tests := []struct {
		name    string
		section string
		wantErr bool
	}{
		{
			name:    "invalid body section",
			section: "body",
			wantErr: true,
		},
		{
			name:    "invalid claims section",
			section: "claims",
			wantErr: true,
		},
		{
			name:    "uppercase HEADER",
			section: "HEADER",
			wantErr: true,
		},
		{
			name:    "uppercase PAYLOAD",
			section: "PAYLOAD",
			wantErr: true,
		},
		{
			name:    "valid header",
			section: "header",
			wantErr: false,
		},
		{
			name:    "valid payload",
			section: "payload",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Claims: []ClaimMapping{
					{ClaimPath: "sub", HeaderName: "X-User-Id"},
				},
				Sections:      []string{tt.section},
				MaxClaimDepth: 10,
				MaxHeaderSize: 8192,
			}

			err := config.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() expected error for section=%q, got nil", tt.section)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() unexpected error for section=%q: %v", tt.section, err)
			}
		})
	}
}

// TestValidate_ZeroMaxClaimDepth verifies error for zero maxClaimDepth
func TestValidate_ZeroMaxClaimDepth(t *testing.T) {
	config := &Config{
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:      []string{"payload"},
		MaxClaimDepth: 0,
		MaxHeaderSize: 8192,
	}

	err := config.Validate()
	if err == nil {
		t.Errorf("Validate() expected error for zero maxClaimDepth, got nil")
	}
}

// TestValidate_NegativeMaxClaimDepth verifies error for negative maxClaimDepth
func TestValidate_NegativeMaxClaimDepth(t *testing.T) {
	config := &Config{
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:      []string{"payload"},
		MaxClaimDepth: -1,
		MaxHeaderSize: 8192,
	}

	err := config.Validate()
	if err == nil {
		t.Errorf("Validate() expected error for negative maxClaimDepth, got nil")
	}
}

// TestValidate_ZeroMaxHeaderSize verifies error for zero maxHeaderSize
func TestValidate_ZeroMaxHeaderSize(t *testing.T) {
	config := &Config{
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:      []string{"payload"},
		MaxClaimDepth: 10,
		MaxHeaderSize: 0,
	}

	err := config.Validate()
	if err == nil {
		t.Errorf("Validate() expected error for zero maxHeaderSize, got nil")
	}
}

// TestValidate_NegativeMaxHeaderSize verifies error for negative maxHeaderSize
func TestValidate_NegativeMaxHeaderSize(t *testing.T) {
	config := &Config{
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:      []string{"payload"},
		MaxClaimDepth: 10,
		MaxHeaderSize: -1,
	}

	err := config.Validate()
	if err == nil {
		t.Errorf("Validate() expected error for negative maxHeaderSize, got nil")
	}
}

// TestValidate_MultipleSections verifies validation with multiple valid sections
func TestValidate_MultipleSections(t *testing.T) {
	config := &Config{
		Claims: []ClaimMapping{
			{ClaimPath: "sub", HeaderName: "X-User-Id"},
		},
		Sections:      []string{"payload", "header"},
		MaxClaimDepth: 10,
		MaxHeaderSize: 8192,
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("Validate() failed for multiple valid sections: %v", err)
	}
}

// TestCreateConfig verifies default configuration creation
func TestCreateConfig(t *testing.T) {
	config := CreateConfig()

	if config.SourceHeader != "Authorization" {
		t.Errorf("Default SourceHeader = %q, want Authorization", config.SourceHeader)
	}
	if config.TokenPrefix != "Bearer " {
		t.Errorf("Default TokenPrefix = %q, want 'Bearer '", config.TokenPrefix)
	}
	if len(config.Sections) != 1 || config.Sections[0] != "payload" {
		t.Errorf("Default Sections = %v, want [payload]", config.Sections)
	}
	if !config.ContinueOnError {
		t.Errorf("Default ContinueOnError = %v, want true", config.ContinueOnError)
	}
	if config.RemoveSourceHeader {
		t.Errorf("Default RemoveSourceHeader = %v, want false", config.RemoveSourceHeader)
	}
	if config.MaxClaimDepth != 10 {
		t.Errorf("Default MaxClaimDepth = %d, want 10", config.MaxClaimDepth)
	}
	if config.MaxHeaderSize != 8192 {
		t.Errorf("Default MaxHeaderSize = %d, want 8192", config.MaxHeaderSize)
	}
	if config.LogMissingClaims {
		t.Errorf("Default LogMissingClaims = %v, want false", config.LogMissingClaims)
	}
}

// TestValidate_LogMissingClaims verifies LogMissingClaims field validates correctly
func TestValidate_LogMissingClaims(t *testing.T) {
	tests := []struct {
		name             string
		logMissingClaims bool
		wantErr          bool
	}{
		{
			name:             "LogMissingClaims true",
			logMissingClaims: true,
			wantErr:          false,
		},
		{
			name:             "LogMissingClaims false",
			logMissingClaims: false,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Claims: []ClaimMapping{
					{ClaimPath: "sub", HeaderName: "X-User-Id"},
				},
				Sections:         []string{"payload"},
				MaxClaimDepth:    10,
				MaxHeaderSize:    8192,
				LogMissingClaims: tt.logMissingClaims,
			}

			err := config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
