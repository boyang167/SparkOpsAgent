package application

import (
	"encoding/json"
	"fmt"
)

func parseStringArg(args map[string]any, key string) (string, error) {
	raw, ok := args[key].(string)
	if !ok || raw == "" {
		return "", fmt.Errorf("missing required argument: %s", key)
	}
	return raw, nil
}

func parseIntArg(args map[string]any, key string, defaultValue int) int {
	switch v := args[key].(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	case json.Number:
		if n, err := v.Int64(); err == nil && n > 0 {
			return int(n)
		}
	}
	return defaultValue
}
