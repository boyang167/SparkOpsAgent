package domain

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func AllTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "list_projects",
			Description: "List active projects in the current workspace.",
			InputSchema: objectSchema(map[string]any{}),
		},
		{
			Name:        "list_knowledge_bases",
			Description: "List knowledge bases available in the current workspace.",
			InputSchema: objectSchema(map[string]any{}),
		},
		{
			Name:        "list_agents",
			Description: "List agents under a project.",
			InputSchema: objectSchema(map[string]any{
				"project_id": stringProperty("Project UUID"),
			}, "project_id"),
		},
		{
			Name:        "search_knowledge",
			Description: "Search documents in a knowledge base.",
			InputSchema: objectSchema(map[string]any{
				"knowledge_base_id": stringProperty("Knowledge base UUID"),
				"query":             stringProperty("Search query"),
				"limit":             integerProperty("Max results (default 5)"),
			}, "knowledge_base_id", "query"),
		},
		{
			Name:        "chat_with_agent",
			Description: "Send a message to a published agent and wait for the reply.",
			InputSchema: objectSchema(map[string]any{
				"agent_id":        stringProperty("Agent UUID"),
				"message":         stringProperty("User message"),
				"conversation_id": stringProperty("Optional conversation UUID to continue"),
			}, "agent_id", "message"),
		},
	}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integerProperty(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}
