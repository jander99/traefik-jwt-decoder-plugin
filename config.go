package traefik_jwt_decoder_plugin

import (
	"fmt"
	"strings"
)

// Config holds the complete plugin configuration with all settings for
// JWT parsing, claim extraction, and header injection behavior.
type Config struct {
	// SourceHeader is the HTTP header to extract JWT from (default: "Authorization")
	SourceHeader string `json:"sourceHeader,omitempty" yaml:"sourceHeader,omitempty"`

	// TokenPrefix is the prefix to strip from token value (default: "Bearer ")
	// Set to empty string if no prefix stripping is needed
	TokenPrefix string `json:"tokenPrefix,omitempty" yaml:"tokenPrefix,omitempty"`

	// Claims is the list of claim-to-header mappings to process
	// Must contain at least one mapping
	Claims []ClaimMapping `json:"claims,omitempty" yaml:"claims,omitempty"`

	// Sections specifies which JWT sections to read claims from:
	//   - ["payload"]: Only read from payload (default)
	//   - ["header"]: Only read from JWT header
	//   - ["payload", "header"]: Try payload first, fallback to header
	Sections []string `json:"sections,omitempty" yaml:"sections,omitempty"`

	// ContinueOnError determines error handling behavior:
	//   - true (default): Log errors and pass request through
	//   - false: Return 401 Unauthorized on JWT errors
	ContinueOnError bool `json:"continueOnError,omitempty" yaml:"continueOnError,omitempty"`

	// RemoveSourceHeader removes the source header after processing (default: false)
	// Useful for preventing JWT exposure to upstream services
	RemoveSourceHeader bool `json:"removeSourceHeader,omitempty" yaml:"removeSourceHeader,omitempty"`

	// MaxClaimDepth is the maximum depth for nested claim paths (default: 10)
	// Prevents deep recursion attacks
	MaxClaimDepth int `json:"maxClaimDepth,omitempty" yaml:"maxClaimDepth,omitempty"`

	// MaxHeaderSize is the maximum size of header values in bytes (default: 8192)
	// Prevents memory exhaustion attacks
	MaxHeaderSize int `json:"maxHeaderSize,omitempty" yaml:"maxHeaderSize,omitempty"`

	// LogLevel controls logging verbosity (default: "warn")
	// Valid values: "debug", "info", "warn", "error"
	//   - "debug": Log all operations including header injections
	//   - "info": Log significant operations
	//   - "warn": Log warnings and errors only (production default)
	//   - "error": Log errors only
	LogLevel string `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`

	// StrictMode validates JWT header structure (requires 'alg' field) (default: false)
	// Set to true for enhanced security validation, false for backward compatibility
	StrictMode bool `json:"strictMode,omitempty" yaml:"strictMode,omitempty"`

	// LogMissingClaims controls whether to log when claims are not found (default: false)
	// Set to true for debugging, false for production to reduce log noise
	LogMissingClaims bool `json:"logMissingClaims,omitempty" yaml:"logMissingClaims,omitempty"`
}

// ClaimMapping defines a single mapping from a JWT claim path to an HTTP header name.
type ClaimMapping struct {
	// ClaimPath is the path to the claim using dot notation (e.g., "user.profile.email")
	// Required field
	ClaimPath string `json:"claimPath" yaml:"claimPath"`

	// HeaderName is the target HTTP header name (e.g., "X-User-Email")
	// Required field
	HeaderName string `json:"headerName" yaml:"headerName"`

	// Override determines behavior when header already exists:
	//   - false (default): Preserve existing header
	//   - true: Replace existing header with claim value
	Override bool `json:"override,omitempty" yaml:"override,omitempty"`

	// ArrayFormat specifies how to format array claims:
	//   - "comma" (default): ["admin", "user"] → "admin, user"
	//   - "json": ["admin", "user"] → "[\"admin\",\"user\"]"
	ArrayFormat string `json:"arrayFormat,omitempty" yaml:"arrayFormat,omitempty"`
}

// CreateConfig creates and initializes the plugin configuration with default values.
// Called by Traefik during plugin initialization.
func CreateConfig() *Config {
	return &Config{
		SourceHeader:       "Authorization",
		TokenPrefix:        "Bearer ",
		Claims:             []ClaimMapping{},
		Sections:           []string{"payload"},
		ContinueOnError:    true,
		RemoveSourceHeader: false,
		MaxClaimDepth:      10,
		MaxHeaderSize:      8192,
		LogLevel:           "warn",
		StrictMode:         false,
		LogMissingClaims:   false,
	}
}

// Validate checks the configuration for errors and enforces business rules.
// Called during plugin initialization to ensure configuration is valid before
// processing any requests.
//
// Validation Rules:
//   - Claims array must not be empty
//   - Each ClaimMapping must have non-empty claimPath and headerName
//   - ArrayFormat must be "", "comma", or "json"
//   - No duplicate headerName values (case-insensitive)
//   - Sections must contain only "header" or "payload"
//   - Sections array must not be empty
//   - MaxClaimDepth must be greater than 0
//   - MaxHeaderSize must be greater than 0
//
// Returns descriptive error if any validation rule is violated.
func (c *Config) Validate() error {
	// Check Claims array not empty
	if len(c.Claims) == 0 {
		return fmt.Errorf("claims array cannot be empty")
	}

	// Track header names for duplicate detection (case-insensitive)
	headerNames := make(map[string]bool)

	// Validate each ClaimMapping
	for i, claim := range c.Claims {
		// ClaimPath must not be empty
		if claim.ClaimPath == "" {
			return fmt.Errorf("claim mapping %d: claimPath is required", i)
		}

		// HeaderName must not be empty
		if claim.HeaderName == "" {
			return fmt.Errorf("claim mapping %d: headerName is required", i)
		}

		// ArrayFormat must be "", "comma", or "json"
		if claim.ArrayFormat != "" && claim.ArrayFormat != "comma" && claim.ArrayFormat != "json" {
			return fmt.Errorf("claim mapping %d: invalid arrayFormat '%s', must be 'comma' or 'json'", i, claim.ArrayFormat)
		}

		// Check for duplicate header names (case-insensitive)
		lowerHeaderName := strings.ToLower(claim.HeaderName)
		if headerNames[lowerHeaderName] {
			return fmt.Errorf("duplicate headerName: %s", claim.HeaderName)
		}
		headerNames[lowerHeaderName] = true
	}

	// Validate Sections array
	if len(c.Sections) == 0 {
		return fmt.Errorf("sections array cannot be empty")
	}

	for _, section := range c.Sections {
		if section != "header" && section != "payload" {
			return fmt.Errorf("invalid section '%s', must be 'header' or 'payload'", section)
		}
	}

	// Check MaxClaimDepth > 0
	if c.MaxClaimDepth <= 0 {
		return fmt.Errorf("maxClaimDepth must be greater than 0")
	}

	// Check MaxHeaderSize > 0
	if c.MaxHeaderSize <= 0 {
		return fmt.Errorf("maxHeaderSize must be greater than 0")
	}

	// Validate LogLevel if provided
	if c.LogLevel != "" {
		validLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		if !validLevels[c.LogLevel] {
			return fmt.Errorf("invalid logLevel '%s', must be 'debug', 'info', 'warn', or 'error'", c.LogLevel)
		}
	}

	return nil
}
