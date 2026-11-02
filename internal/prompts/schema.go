package prompts

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
)

//go:embed schemas/*.json
var schemaFiles embed.FS

func Schema(name string) ([]byte, error) {
	data, err := schemaFiles.ReadFile("schemas/" + name + ".schema.json")
	if err != nil {
		return nil, fmt.Errorf("schema %q is not registered: %w", name, err)
	}
	return data, nil
}

func ValidateNamedJSON(data []byte, name string) error {
	schemaData, err := Schema(name)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return fmt.Errorf("invalid schema JSON: %w", err)
	}
	return validateValue(value, schema, "$", false)
}

// ValidateJSON performs the deterministic part of structured-output
// validation: JSON syntax, object shape, required fields, primitive types,
// enums, and string/array bounds. Semantic Japanese correctness remains a
// model-evaluation concern.
func ValidateJSON(data []byte, schemaPath string) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return fmt.Errorf("invalid schema JSON: %w", err)
	}

	if err := validateValue(value, schema, "$", false); err != nil {
		return err
	}

	return nil
}

func validateValue(value any, schema map[string]any, path string, nullable bool) error {
	if value == nil {
		if nullable {
			return nil
		}
		return fmt.Errorf("%s must not be null", path)
	}
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, candidate := range enum {
			if fmt.Sprint(candidate) == fmt.Sprint(value) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s is not an allowed value", path)
		}
	}
	typeName, _ := schema["type"].(string)
	switch typeName {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}
		properties, _ := schema["properties"].(map[string]any)
		required, _ := schema["required"].([]any)
		for _, item := range required {
			name := fmt.Sprint(item)
			if _, ok := object[name]; !ok {
				return fmt.Errorf("%s.%s is required", path, name)
			}
		}
		additional, exists := schema["additionalProperties"].(bool)
		for name, item := range object {
			definition, known := properties[name]
			if !known && exists && !additional {
				return fmt.Errorf("%s.%s is not allowed", path, name)
			}
			if known {
				child, _ := definition.(map[string]any)
				if err := validateValue(item, child, path+"."+name, false); err != nil {
					return err
				}
			}
		}
	case "array":
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		if max, ok := number(schema["maxItems"]); ok && len(array) > int(max) {
			return fmt.Errorf("%s has too many items", path)
		}
		if min, ok := number(schema["minItems"]); ok && len(array) < int(min) {
			return fmt.Errorf("%s has too few items", path)
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, item := range array {
				if err := validateValue(item, items, fmt.Sprintf("%s[%d]", path, i), false); err != nil {
					return err
				}
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", path)
		}
		if max, ok := number(schema["maxLength"]); ok && len([]rune(text)) > int(max) {
			return fmt.Errorf("%s is too long", path)
		}
		if min, ok := number(schema["minLength"]); ok && len([]rune(text)) < int(min) {
			return fmt.Errorf("%s is too short", path)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("%s must be a number", path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", path)
		}
	}
	return nil
}

func number(value any) (float64, bool) { number, ok := value.(float64); return number, ok }

func ValidSchema(data []byte) bool {
	var value any
	return json.NewDecoder(bytes.NewReader(data)).Decode(&value) == nil
}
