package traefik_jwt_decoder_plugin

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

// JWTClaimsHeaders is the main plugin struct implementing the http.Handler interface.
// It orchestrates JWT parsing, claim extraction, and header injection for each request.
//
// The struct is immutable after creation, making it thread-safe for concurrent requests.
type JWTClaimsHeaders struct {
	// next is the next HTTP handler in the Traefik middleware chain
	next http.Handler

	// config holds the validated plugin configuration (immutable)
	config *Config

	// name is the plugin instance name for logging
	name string
}

// New creates a new JWTClaimsHeaders plugin instance.
// This function is called by Traefik during plugin initialization.
//
// The configuration is validated before creating the plugin instance.
// If validation fails, an error is returned and the plugin won't be loaded.
//
// Parameters:
//   - ctx: Context for initialization (currently unused)
//   - next: Next handler in the middleware chain
//   - config: Plugin configuration (will be validated)
//   - name: Plugin instance name for logging
//
// Returns the plugin instance or an error if configuration is invalid.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &JWTClaimsHeaders{
		next:   next,
		config: config,
		name:   name,
	}, nil
}

// ServeHTTP implements the http.Handler interface to process each HTTP request.
// This is the main entry point for request processing in the middleware chain.
//
// Request Processing Flow:
//   1. Extract JWT from source header
//   2. Parse JWT (base64url decode, JSON unmarshal)
//   3. For each claim mapping:
//      a. Try extracting claim from configured sections
//      b. Convert claim value to string
//      c. Inject as HTTP header (with security guards)
//   4. Optionally remove source header
//   5. Forward request to next handler
//
// Error Handling:
//   - If continueOnError=true: Log errors and pass request through
//   - If continueOnError=false: Return 401 Unauthorized with JSON error body
//
// Thread Safety:
//   - All data flows through function parameters (no shared state)
//   - Safe for concurrent execution across multiple requests
func (j *JWTClaimsHeaders) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// 1. Extract JWT from source header
	headerValue := req.Header.Get(j.config.SourceHeader)
	if headerValue == "" {
		log.Printf("[%s] JWT source header not found: %s", j.name, j.config.SourceHeader)
		if j.config.ContinueOnError {
			j.next.ServeHTTP(rw, req)
			return
		}
		j.returnError(rw, "unauthorized", "missing JWT token")
		return
	}

	// 2. Extract token (strip prefix)
	token := ExtractToken(headerValue, j.config.TokenPrefix)

	// 3. Parse JWT
	jwt, err := ParseJWT(token)
	if err != nil {
		log.Printf("[%s] JWT parse error: %v", j.name, err)
		if j.config.ContinueOnError {
			j.next.ServeHTTP(rw, req)
			return
		}
		j.returnError(rw, "unauthorized", "invalid JWT token")
		return
	}

	// 4. Process each claim mapping
	for _, claimMapping := range j.config.Claims {
		// Determine which sections to search
		var claimValue interface{}
		var found bool

		for _, section := range j.config.Sections {
			var data map[string]interface{}
			if section == "payload" {
				data = jwt.Payload
			} else if section == "header" {
				data = jwt.Header
			}

			// Try to extract claim from this section
			value, err := ExtractClaim(data, claimMapping.ClaimPath, j.config.MaxClaimDepth)
			if err == nil {
				claimValue = value
				found = true
				break // Found in this section, stop searching
			}
		}

		if !found {
			if j.config.LogMissingClaims {
				log.Printf("[%s] Claim not found: %s", j.name, claimMapping.ClaimPath)
			}
			continue // Skip this mapping
		}

		// Convert claim to string
		strValue, err := ConvertClaimToString(claimValue, claimMapping.ArrayFormat)
		if err != nil {
			log.Printf("[%s] Failed to convert claim %s: %v", j.name, claimMapping.ClaimPath, err)
			continue
		}

		// Inject header
		err = InjectHeader(req, claimMapping.HeaderName, strValue, claimMapping.Override, j.config.MaxHeaderSize)
		if err != nil {
			log.Printf("[%s] Failed to inject header %s: %v", j.name, claimMapping.HeaderName, err)
			continue
		}

		log.Printf("[%s] Injected header: %s = %s", j.name, claimMapping.HeaderName, strValue)
	}

	// 5. Remove source header if configured
	if j.config.RemoveSourceHeader {
		req.Header.Del(j.config.SourceHeader)
	}

	// 6. Forward to next handler
	j.next.ServeHTTP(rw, req)
}

// returnError sends a JSON error response with 401 Unauthorized status.
// Used when continueOnError=false and JWT processing fails.
//
// Response format:
//   {
//     "error": "<errorType>",
//     "message": "<message>"
//   }
//
// Parameters:
//   - rw: Response writer to send error response
//   - errorType: Error type identifier (e.g., "unauthorized")
//   - message: Human-readable error message
func (j *JWTClaimsHeaders) returnError(rw http.ResponseWriter, errorType, message string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusUnauthorized)

	errorResponse := map[string]string{
		"error":   errorType,
		"message": message,
	}

	json.NewEncoder(rw).Encode(errorResponse)
}
