package kernel

import (
	"fmt"
	"math"
	"sort"
)

// The kernel validates operation input against a deliberately small
// JSON-Schema subset: type, properties, required, additionalProperties,
// items, and enum. Anything else in a schema is rejected at registration so a
// package can never believe a constraint is enforced when it is not.

var supportedSchemaKeys = map[string]bool{
	"type": true, "properties": true, "required": true, "additionalProperties": true,
	"items": true, "enum": true, "description": true,
}

// checkSchema verifies that a schema only uses supported keywords
func checkSchema(schema map[string]any) error {
	for key := range schema {
		if !supportedSchemaKeys[key] {
			return fmt.Errorf("unsupported keyword %q", key)
		}
	}
	if t, ok := schema["type"]; ok {
		switch t {
		case "object", "array", "string", "number", "integer", "boolean":
		default:
			return fmt.Errorf("unsupported type %v", t)
		}
	}
	if ap, ok := schema["additionalProperties"]; ok {
		if _, isBool := ap.(bool); !isBool {
			return fmt.Errorf("additionalProperties must be a boolean")
		}
	}
	if req, ok := schema["required"]; ok {
		if _, err := toStringSlice(req); err != nil {
			return fmt.Errorf("required: %v", err)
		}
	}
	if props, ok := schema["properties"]; ok {
		pm, ok := props.(map[string]any)
		if !ok {
			return fmt.Errorf("properties must be an object")
		}
		for name, sub := range pm {
			sm, ok := sub.(map[string]any)
			if !ok {
				return fmt.Errorf("properties.%s must be an object", name)
			}
			if err := checkSchema(sm); err != nil {
				return fmt.Errorf("properties.%s: %v", name, err)
			}
		}
	}
	if items, ok := schema["items"]; ok {
		im, ok := items.(map[string]any)
		if !ok {
			return fmt.Errorf("items must be an object")
		}
		if err := checkSchema(im); err != nil {
			return fmt.Errorf("items: %v", err)
		}
	}
	return nil
}

// validateValue checks a decoded JSON value against a schema. path is used
// to build readable error messages such as "input.path: expected string".
func validateValue(schema map[string]any, value any, path string) error {
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			if e == value {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s: value is not one of the allowed values", path)
		}
	}

	switch schema["type"] {
	case nil:
		return nil
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: expected string", path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean", path)
		}
	case "number":
		if _, ok := toFloat(value); !ok {
			return fmt.Errorf("%s: expected number", path)
		}
	case "integer":
		f, ok := toFloat(value)
		if !ok || f != math.Trunc(f) {
			return fmt.Errorf("%s: expected integer", path)
		}
	case "array":
		arr, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array", path)
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, v := range arr {
				if err := validateValue(items, v, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		return validateObject(schema, obj, path)
	}
	return nil
}

func validateObject(schema map[string]any, obj map[string]any, path string) error {
	required, _ := toStringSlice(schema["required"])
	for _, name := range required {
		if _, ok := obj[name]; !ok {
			return fmt.Errorf("%s.%s: is required", path, name)
		}
	}

	props, _ := schema["properties"].(map[string]any)
	allowExtra, hasRule := schema["additionalProperties"].(bool)
	closed := hasRule && !allowExtra

	// Sorted for deterministic error messages
	names := make([]string, 0, len(obj))
	for name := range obj {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		sub, declared := props[name].(map[string]any)
		if !declared {
			if closed {
				return fmt.Errorf("%s.%s: is not an allowed property", path, name)
			}
			continue
		}
		if err := validateValue(sub, obj[name], path+"."+name); err != nil {
			return err
		}
	}
	return nil
}

func toStringSlice(v any) ([]string, error) {
	switch s := v.(type) {
	case nil:
		return nil, nil
	case []string:
		return s, nil
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			str, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf("must be a list of strings")
			}
			out = append(out, str)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("must be a list of strings")
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}
