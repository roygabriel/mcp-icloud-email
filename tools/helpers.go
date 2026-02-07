package tools

import (
	"fmt"
	"net/mail"
)

// parseAddressList extracts a string or []interface{} argument into a validated email address list.
// Returns a non-nil error if the value is present but invalid.
func parseAddressList(args map[string]interface{}, key string) ([]string, error) {
	val, ok := args[key]
	if !ok || val == nil {
		return nil, nil
	}

	var raw []string
	switch v := val.(type) {
	case string:
		if v != "" {
			raw = []string{v}
		}
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				raw = append(raw, str)
			}
		}
	default:
		return nil, fmt.Errorf("%s must be a string or array of strings", key)
	}

	// Validate each address
	for _, addr := range raw {
		if _, err := mail.ParseAddress(addr); err != nil {
			return nil, fmt.Errorf("invalid %s email address '%s': %v", key, addr, err)
		}
	}

	return raw, nil
}

// requireAddressList is like parseAddressList but returns an error if the result is empty.
func requireAddressList(args map[string]interface{}, key string) ([]string, error) {
	addrs, err := parseAddressList(args, key)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("%s is required", key)
	}
	return addrs, nil
}
