package resources

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
)

const healthUnknown = "Unknown"

type Health struct {
	Status         string `json:"status"`
	Stale          bool   `json:"stale"`
	ReportedStatus string `json:"reportedStatus,omitempty"`
}

type Condition struct {
	Type               string `json:"type,omitempty"`
	Status             string `json:"status,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	Stale              bool   `json:"stale"`
}

type Presentation struct {
	Health     Health      `json:"health"`
	Conditions []Condition `json:"conditions"`
}

func present(contents []byte) Presentation {
	result := Presentation{Health: Health{Status: healthUnknown}, Conditions: []Condition{}}
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	if decoder.Decode(&object) != nil {
		return result
	}
	generation, hasGeneration := integerAt(object, "metadata", "generation")
	status, ok := object["status"].(map[string]any)
	if !ok {
		return result
	}
	conditions, ok := status["conditions"].([]any)
	if !ok {
		return result
	}
	readyFound := false
	for _, raw := range conditions {
		fields, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		condition := Condition{
			Type:    stringField(fields, "type"),
			Status:  stringField(fields, "status"),
			Reason:  stringField(fields, "reason"),
			Message: stringField(fields, "message"),
		}
		if observed, exists := integer(fields["observedGeneration"]); exists {
			condition.ObservedGeneration = &observed
			condition.Stale = hasGeneration && observed < generation
		}
		result.Conditions = append(result.Conditions, condition)
		if readyFound || condition.Type != "Ready" {
			continue
		}
		readyFound = true
		result.Health.ReportedStatus = condition.Status
		result.Health.Stale = condition.Stale
		switch condition.Status {
		case "True":
			result.Health.Status = "Healthy"
		case "False":
			result.Health.Status = "Unhealthy"
		default:
			result.Health.Status = healthUnknown
		}
	}
	return result
}

func stringField(fields map[string]any, name string) string {
	value, ok := fields[name].(string)
	if !ok {
		return ""
	}
	return value
}

func integerAt(object map[string]any, path ...string) (int64, bool) {
	var current any = object
	for _, element := range path {
		fields, ok := current.(map[string]any)
		if !ok {
			return 0, false
		}
		current = fields[element]
	}
	return integer(current)
}

func integer(value any) (int64, bool) {
	switch number := value.(type) {
	case json.Number:
		parsed, err := strconv.ParseInt(string(number), 10, 64)
		return parsed, err == nil
	case int64:
		return number, true
	case float64:
		if math.Trunc(number) == number && number >= math.MinInt64 && number <= math.MaxInt64 {
			return int64(number), true
		}
	}
	return 0, false
}
