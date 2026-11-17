package evolution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"reflect"
	"strings"
)

// CheckerHash 冻结服务器实现；候选和客户端不能替换检查器或获得Expected。
func CheckerHash() string {
	return hashText("pangu/eino_json_v1/checker/json_equal/v1;object;no-duplicates;64KiB")
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func hashJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return hashText(string(data))
}

func checkJSONResult(expected string, criteria []string, actual string) (bool, error) {
	if len(criteria) != 1 || criteria[0] != "json_equal" {
		return false, ErrChecker
	}
	want, err := strictObject(expected)
	if err != nil {
		return false, ErrChecker
	}
	got, err := strictObject(actual)
	if err != nil {
		return false, nil
	}
	return reflect.DeepEqual(want, got), nil
}

func strictObject(raw string) (map[string]any, error) {
	if len(raw) > 65536 {
		return nil, ErrChecker
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	value, err := strictValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, ErrChecker
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, ErrChecker
	}
	return object, nil
}

func strictValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, ErrChecker
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		return strictMap(decoder, depth+1)
	case '[':
		values := []any{}
		for decoder.More() {
			value, err := strictValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		if _, err = decoder.Token(); err != nil {
			return nil, err
		}
		return values, nil
	default:
		return nil, ErrChecker
	}
}

func strictMap(decoder *json.Decoder, depth int) (map[string]any, error) {
	object := map[string]any{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if !ok {
			return nil, ErrChecker
		}
		if _, exists := object[name]; exists {
			return nil, ErrChecker
		}
		value, err := strictValue(decoder, depth)
		if err != nil {
			return nil, err
		}
		object[name] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return object, nil
}
