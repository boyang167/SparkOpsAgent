package keyval

import (
	"encoding/json"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
)

// Resolve substitutes {{key}} placeholders in definition JSON using key_vals values.
func Resolve(definition json.RawMessage, keyVals []model.KeyVal) (json.RawMessage, error) {
	if len(definition) == 0 {
		return json.RawMessage("{}"), nil
	}
	text := string(definition)
	for _, kv := range keyVals {
		if kv.Key == "" {
			continue
		}
		text = strings.ReplaceAll(text, "{{"+kv.Key+"}}", kv.Value)
	}
	if !json.Valid([]byte(text)) {
		return nil, ErrInvalidResolvedDefinition
	}
	return json.RawMessage(text), nil
}

// MergeKeyVals overlays runtime values onto template defaults by key.
func MergeKeyVals(base []model.KeyVal, overrides map[string]string) []model.KeyVal {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}
	out := make([]model.KeyVal, len(base))
	copy(out, base)
	index := make(map[string]int, len(out))
	for i, kv := range out {
		index[kv.Key] = i
	}
	for key, value := range overrides {
		if i, ok := index[key]; ok {
			out[i].Value = value
			continue
		}
		out = append(out, model.KeyVal{Key: key, Value: value})
	}
	return out
}
