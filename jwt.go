// Package traefik_jwt_decoder_plugin implements a Traefik middleware that extracts
// claims from JWT tokens and injects them as HTTP headers for upstream services.
//
// This plugin performs JWT parsing without signature verification. It is designed
// for internal service-to-service communication where JWT validation occurs at the
// edge (API gateway) and internal services need access to JWT claims.
//
// Security Warning: This plugin does NOT verify JWT signatures. Deploy only behind
// authenticated API gateways in trusted internal networks.
package traefik_jwt_decoder_plugin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// JWT represents a parsed JWT token with decoded header and payload sections.
// The signature is stored as a string but is NOT verified by this plugin.
type JWT struct {
	// Header contains decoded JWT header claims (typ, alg, kid, etc.)
	Header map[string]interface{}

	// Payload contains decoded JWT payload claims (sub, email, roles, etc.)
	Payload map[string]interface{}

	// Signature is the base64url-encoded signature (not decoded or verified)
	Signature string
}

// ParseJWT decodes a JWT token without signature verification.
// This function is designed for scenarios where JWT validation occurs
// at the edge and internal services only need claim extraction.
//
// The token must be in the format: header.payload.signature
// Both header and payload are base64url-encoded JSON objects.
// The signature is stored but NOT validated.
//
// Security Note: This function does NOT verify JWT signatures.
// It should only be used in trusted internal networks where
// signature validation happens at the API gateway.
//
// Strict Mode: When enabled, validates JWT header contains required 'alg' field.
// This helps detect malformed tokens that might indicate attacks or misconfigurations.
//
// Example:
//   jwt, err := ParseJWT("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", false)
//   if err != nil {
//       return nil, fmt.Errorf("parse failed: %w", err)
//   }
//   userID := jwt.Payload["sub"]
//
// Returns an error if:
//   - Token format is invalid (not exactly 3 segments)
//   - Base64 decoding fails for header or payload
//   - JSON parsing fails for header or payload
//   - strictMode=true and 'alg' field is missing from JWT header
func ParseJWT(token string, strictMode bool) (*JWT, error) {
	// Split token into segments
	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 segments, got %d", len(segments))
	}

	// Decode header (segment 0)
	headerBytes, err := base64.RawURLEncoding.DecodeString(segments[0])
	if err != nil {
		return nil, fmt.Errorf("invalid JWT encoding: %v", err)
	}

	// Decode payload (segment 1)
	payloadBytes, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		return nil, fmt.Errorf("invalid JWT encoding: %v", err)
	}

	// Parse header JSON
	var header map[string]interface{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("invalid JWT JSON: %v", err)
	}

	// Parse payload JSON
	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("invalid JWT JSON: %v", err)
	}

	// Validate JWT header structure in strict mode
	if strictMode {
		if _, ok := header["alg"]; !ok {
			return nil, fmt.Errorf("invalid JWT header: missing required 'alg' field")
		}
	}

	// Return JWT struct with signature as-is (not decoded)
	return &JWT{
		Header:    header,
		Payload:   payload,
		Signature: segments[2],
	}, nil
}

// ExtractToken removes a configured prefix from a token value.
// Commonly used to strip "Bearer " from Authorization header values.
//
// If prefix is empty, returns value unchanged.
// If value doesn't start with prefix, returns value unchanged.
// Otherwise, strips prefix and trims whitespace from result.
//
// Example:
//   token := ExtractToken("Bearer eyJhbGc...", "Bearer ")
//   // Returns: "eyJhbGc..."
//
//   token := ExtractToken("eyJhbGc...", "")
//   // Returns: "eyJhbGc..." (no prefix to strip)
func ExtractToken(value, prefix string) string {
	// Handle empty prefix case
	if prefix == "" {
		return value
	}

	// Check if value starts with prefix
	if !strings.HasPrefix(value, prefix) {
		return value
	}

	// Strip prefix and trim whitespace
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}
