package intelligence

import (
	"bytes"
	"encoding/json"
	"io"
)

var (
	resultFields = []string{"reply", "intent", "actions"}
	actionFields = []string{"operation", "entry_id", "entry", "evidence"}
	valueFields  = []string{"kind", "label", "amount", "calories", "protein_g", "carbs_g", "fat_g", "duration_minutes", "distance_km", "source", "notes"}
)

func decodeResult(data []byte) (Result, error) {
	if err := uniqueJSON(data); err != nil {
		return Result{}, ErrInvalidResult
	}
	root, err := exactObject(data, resultFields)
	if err != nil || !nonNull(root, resultFields...) {
		return Result{}, ErrInvalidResult
	}
	var actions []json.RawMessage
	if err := json.Unmarshal(root["actions"], &actions); err != nil {
		return Result{}, ErrInvalidResult
	}
	for _, raw := range actions {
		action, err := exactObject(raw, actionFields)
		if err != nil || !nonNull(action, "operation", "evidence") {
			return Result{}, ErrInvalidResult
		}
		if !bytes.Equal(action["entry"], []byte("null")) {
			entry, err := exactObject(action["entry"], valueFields)
			if err != nil || !nonNull(entry, "kind", "label", "amount", "source", "notes") {
				return Result{}, ErrInvalidResult
			}
		}
	}
	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return Result{}, ErrInvalidResult
	}
	return result, nil
}

func exactObject(data []byte, fields []string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || len(object) != len(fields) {
		return nil, ErrInvalidResult
	}
	for _, field := range fields {
		if _, exists := object[field]; !exists {
			return nil, ErrInvalidResult
		}
	}
	return object, nil
}

func nonNull(object map[string]json.RawMessage, fields ...string) bool {
	for _, field := range fields {
		if bytes.Equal(object[field], []byte("null")) {
			return false
		}
	}
	return true
}

// encoding/json otherwise accepts duplicate keys and case-insensitive struct
// fields. Exact field maps above and this walk remove that ambiguity.
func uniqueJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := uniqueValue(decoder, 0); err != nil {
		return ErrInvalidResult
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ErrInvalidResult
	}
	return nil
}

func uniqueValue(decoder *json.Decoder, depth int) error {
	if depth > 32 {
		return ErrInvalidResult
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return ErrInvalidResult
	}
	seen := make(map[string]bool)
	for decoder.More() {
		if delimiter == '{' {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return ErrInvalidResult
			}
			seen[name] = true
		}
		if err := uniqueValue(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}
