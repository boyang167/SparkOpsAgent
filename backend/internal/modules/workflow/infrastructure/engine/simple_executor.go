package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Step struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

type Definition struct {
	Steps []Step `json:"steps"`
}

type Result struct {
	Logs []string `json:"logs"`
}

func Execute(definitionJSON string) (string, error) {
	var def Definition
	if strings.TrimSpace(definitionJSON) == "" || definitionJSON == "{}" {
		def = Definition{Steps: []Step{{Type: "noop"}}}
	} else if err := json.Unmarshal([]byte(definitionJSON), &def); err != nil {
		return "", fmt.Errorf("invalid workflow definition: %w", err)
	}
	if len(def.Steps) == 0 {
		def.Steps = []Step{{Type: "noop"}}
	}

	logs := make([]string, 0, len(def.Steps))
	for i, step := range def.Steps {
		switch step.Type {
		case "log":
			msg := step.Message
			if msg == "" {
				msg = "empty log"
			}
			logs = append(logs, fmt.Sprintf("[step %d][log] %s", i+1, msg))
		case "noop":
			logs = append(logs, fmt.Sprintf("[step %d][noop] ok", i+1))
		default:
			return "", fmt.Errorf("unsupported step type: %s", step.Type)
		}
	}

	out, err := json.Marshal(Result{Logs: logs})
	if err != nil {
		return "", err
	}
	return string(out), nil
}
