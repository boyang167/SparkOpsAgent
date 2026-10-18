package domain

func ObjectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func StringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func IntegerProperty(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}
