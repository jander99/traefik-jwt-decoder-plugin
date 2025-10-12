package traefik_jwt_decoder_plugin

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ExtractClaim navigates a nested map using dot notation to extract a claim value.
// Supports nested object navigation like "user.profile.email" to access deeply nested claims.
//
// The function enforces a maximum depth limit to prevent deep recursion attacks.
// Each dot in the path represents one level of nesting.
//
// Example:
//   data := map[string]interface{}{
//       "user": map[string]interface{}{
//           "profile": map[string]interface{}{
//               "email": "test@example.com",
//           },
//       },
//   }
//   value, err := ExtractClaim(data, "user.profile.email", 10)
//   // Returns: "test@example.com", nil
//
// Returns an error if:
//   - Path depth exceeds maxDepth (DoS prevention)
//   - Any intermediate path segment doesn't exist in the data
//   - Any intermediate value is not an object (cannot navigate further)
func ExtractClaim(data map[string]interface{}, path string, maxDepth int) (interface{}, error) {
	parts := strings.Split(path, ".")

	// Validate depth limit
	if len(parts) > maxDepth {
		return nil, fmt.Errorf("claim path depth exceeds maximum (%d)", maxDepth)
	}

	current := data

	// Navigate through each part of the path
	for i, part := range parts {
		value, exists := current[part]
		if !exists {
			return nil, fmt.Errorf("claim not found: %s", path)
		}

		// If this is the last part, return the value
		if i == len(parts)-1 {
			return value, nil
		}

		// Otherwise, type assert to nested map
		nested, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid claim path: '%s' is not an object", part)
		}

		current = nested
	}

	return nil, fmt.Errorf("claim not found: %s", path)
}

// ConvertClaimToString converts a JWT claim value to a string representation.
// Handles various JSON types including primitives, arrays, and objects.
//
// Supported types:
//   - string: Returned as-is
//   - bool: Converted to "true" or "false"
//   - float64: Converted to string without scientific notation
//   - int: Converted to string
//   - []interface{} (arrays): Formatted based on arrayFormat parameter
//   - map[string]interface{} (objects): JSON marshaled
//   - nil: Returns empty string
//
// Array Formatting:
//   - "comma" (default): ["admin", "user"] → "admin, user"
//   - "json": ["admin", "user"] → "[\"admin\",\"user\"]"
//
// Example:
//   // String claim
//   str, _ := ConvertClaimToString("test@example.com", "comma")
//   // Returns: "test@example.com"
//
//   // Array claim with comma format
//   str, _ := ConvertClaimToString([]interface{}{"admin", "user"}, "comma")
//   // Returns: "admin, user"
//
//   // Array claim with JSON format
//   str, _ := ConvertClaimToString([]interface{}{"admin", "user"}, "json")
//   // Returns: "[\"admin\",\"user\"]"
//
//   // Object claim
//   obj := map[string]interface{}{"tenant_id": "123"}
//   str, _ := ConvertClaimToString(obj, "comma")
//   // Returns: "{\"tenant_id\":\"123\"}"
func ConvertClaimToString(value interface{}, arrayFormat string) (string, error) {
	// Handle nil
	if value == nil {
		return "", nil
	}

	// Type switch for different claim types
	switch v := value.(type) {
	case string:
		return v, nil

	case bool:
		return strconv.FormatBool(v), nil

	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil

	case int:
		return strconv.Itoa(v), nil

	case []interface{}:
		// Handle array based on format
		if arrayFormat == "json" {
			jsonBytes, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("failed to marshal array to JSON: %w", err)
			}
			return string(jsonBytes), nil
		}

		// Default to comma-separated format
		var parts []string
		for _, elem := range v {
			elemStr, err := ConvertClaimToString(elem, arrayFormat)
			if err != nil {
				return "", err
			}
			parts = append(parts, elemStr)
		}
		return strings.Join(parts, ", "), nil

	case map[string]interface{}:
		// Marshal objects to JSON
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("failed to marshal object to JSON: %w", err)
		}
		return string(jsonBytes), nil

	default:
		// Fallback for any other types
		return fmt.Sprintf("%v", value), nil
	}
}
