package seaweedfs

import (
	"encoding/json"
	"errors"
	"strings"
)

func normalizeJSONString(raw string) (string, error) {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return "", err
	}

	normalized, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	return string(normalized), nil
}

func policiesSemanticallyEqual(a string, b string) bool {
	na, errA := normalizePolicyForComparison(a)
	nb, errB := normalizePolicyForComparison(b)
	if errA == nil && errB == nil {
		return na == nb
	}

	return strings.TrimSpace(a) == strings.TrimSpace(b)
}

func normalizePolicyForComparison(raw string) (string, error) {
	if !json.Valid([]byte(raw)) {
		return "", errors.New("invalid JSON")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	if policy, ok := value.(map[string]any); ok {
		if statement, ok := policy["Statement"].(map[string]any); ok {
			policy["Statement"] = []any{statement}
		}
		if statements, ok := policy["Statement"].([]any); ok {
			for _, item := range statements {
				if statement, ok := item.(map[string]any); ok {
					for _, key := range []string{"Action", "NotAction", "Resource", "NotResource"} {
						if scalar, ok := statement[key].(string); ok {
							statement[key] = []any{scalar}
						}
					}
				}
			}
		}
	}
	result, err := json.Marshal(value)
	return string(result), err
}
