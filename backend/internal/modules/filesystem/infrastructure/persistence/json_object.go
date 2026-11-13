package persistence

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type jsonObject map[string]any

func (j jsonObject) Value() (driver.Value, error) {
	if j == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]any(j))
}

func (j *jsonObject) Scan(value any) error {
	if value == nil {
		*j = jsonObject{}
		return nil
	}
	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		return errors.New("filesystem: unsupported json object value")
	}
	if len(raw) == 0 {
		*j = jsonObject{}
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	*j = jsonObject(out)
	return nil
}
