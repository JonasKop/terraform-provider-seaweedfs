package seaweedfs

import (
	"encoding/json"
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
	na, errA := normalizeJSONString(a)
	nb, errB := normalizeJSONString(b)
	if errA == nil && errB == nil {
		return na == nb
	}

	return strings.TrimSpace(a) == strings.TrimSpace(b)
}
