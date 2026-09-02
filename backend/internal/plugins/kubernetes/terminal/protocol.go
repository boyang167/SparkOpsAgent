package terminal

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	MaxInputBytes = 64 << 10
	MinCols       = 2
	MaxCols       = 500
	MinRows       = 2
	MaxRows       = 200
)

type ClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

type ServerMessage struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Code    *int   `json:"code,omitempty"`
}

func ParseClientMessage(raw []byte) (ClientMessage, error) {
	if len(raw) > MaxInputBytes+256 {
		return ClientMessage{}, errors.New("terminal message too large")
	}
	var message ClientMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		return ClientMessage{}, err
	}
	switch message.Type {
	case "read":
		if len(message.Data) > MaxInputBytes {
			return ClientMessage{}, errors.New("terminal input too large")
		}
	case "resize":
		if err := ValidateSize(message.Cols, message.Rows); err != nil {
			return ClientMessage{}, err
		}
	default:
		return ClientMessage{}, fmt.Errorf("unknown terminal message type")
	}
	return message, nil
}

func ValidateSize(cols, rows uint16) error {
	if cols < MinCols || cols > MaxCols || rows < MinRows || rows > MaxRows {
		return errors.New("invalid terminal size")
	}
	return nil
}

func ShellCommand() []string {
	return []string{
		"/bin/sh",
		"-c",
		"clear 2>/dev/null || true; if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi",
	}
}
