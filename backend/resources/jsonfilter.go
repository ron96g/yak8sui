package resources

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"k8s.io/client-go/util/jsonpath"
)

type JSONComparison string

const (
	JSONExists JSONComparison = "exists"
	JSONEquals JSONComparison = "equals"
)

type jsonStepKind uint8

const (
	jsonField jsonStepKind = iota
	jsonIndex
	jsonWildcard
)

type jsonStep struct {
	kind  jsonStepKind
	field string
	index int
}

type compiledJSONFilter struct {
	sql        string
	args       []any
	fallback   bool
	steps      []jsonStep
	comparison JSONComparison
	want       any
}

func compileJSONFilter(path, rawValue string, comparison JSONComparison) (*compiledJSONFilter, error) {
	if comparison == "" {
		comparison = JSONExists
	}
	if comparison != JSONExists && comparison != JSONEquals {
		return nil, fmt.Errorf("comparison must be exists or equals")
	}
	if path == "" {
		if comparison == JSONEquals || rawValue != "" {
			return nil, fmt.Errorf("JSONPath is required for a comparison")
		}
		return nil, nil
	}
	steps, err := parseJSONPath(path)
	if err != nil {
		return nil, err
	}
	filter := &compiledJSONFilter{steps: steps, comparison: comparison}
	if comparison == JSONEquals {
		decoder := json.NewDecoder(strings.NewReader(rawValue))
		decoder.UseNumber()
		if err := decoder.Decode(&filter.want); err != nil {
			return nil, fmt.Errorf("value must be one JSON string, number, boolean, or null: %w", err)
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("value must contain exactly one JSON value")
		}
		switch filter.want.(type) {
		case nil, bool, string, json.Number:
		default:
			return nil, fmt.Errorf("value must be a JSON string, number, boolean, or null")
		}
	}
	filter.sql, filter.args, filter.fallback = buildSQLiteJSONPredicate(steps, comparison, filter.want)
	return filter, nil
}

func parseJSONPath(path string) ([]jsonStep, error) {
	if !strings.HasPrefix(path, "$") || strings.TrimSpace(path) != path {
		return nil, fmt.Errorf("path must start with $ and contain no surrounding whitespace")
	}
	_, err := jsonpath.Parse("resource filter", "{"+path+"}")
	if err != nil {
		return nil, fmt.Errorf("invalid JSONPath: %w", err)
	}
	steps, err := scanJSONPath(path)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("path must select a field or array element")
	}
	return steps, nil
}

func scanJSONPath(path string) ([]jsonStep, error) {
	steps := []jsonStep{}
	for position := 1; position < len(path); {
		switch path[position] {
		case '.':
			step, next, err := scanDotField(path, position+1)
			if err != nil {
				return nil, err
			}
			steps, position = append(steps, step), next
		case '[':
			step, next, err := scanBracketStep(path, position)
			if err != nil {
				return nil, err
			}
			steps, position = append(steps, step), next
		default:
			return nil, fmt.Errorf("only field access, array indexes, and array wildcards are supported")
		}
	}
	return steps, nil
}

func scanDotField(path string, position int) (jsonStep, int, error) {
	var field strings.Builder
	for position < len(path) && path[position] != '.' && path[position] != '[' {
		if path[position] == '\\' {
			position++
			if position == len(path) {
				return jsonStep{}, 0, fmt.Errorf("field escape is incomplete")
			}
		}
		field.WriteByte(path[position])
		position++
	}
	if field.Len() == 0 {
		return jsonStep{}, 0, fmt.Errorf("field names cannot be empty")
	}
	if field.String() == "*" {
		return jsonStep{}, 0, fmt.Errorf("object wildcards are not supported")
	}
	return jsonStep{kind: jsonField, field: field.String()}, position, nil
}

func scanBracketStep(path string, position int) (jsonStep, int, error) {
	end := strings.IndexByte(path[position:], ']')
	if end < 0 {
		return jsonStep{}, 0, fmt.Errorf("unterminated array or quoted field access")
	}
	end += position
	content := path[position+1 : end]
	if content == "*" {
		return jsonStep{kind: jsonWildcard}, end + 1, nil
	}
	if len(content) >= 2 && (content[0] == '\'' || content[0] == '"') && content[len(content)-1] == content[0] {
		field, err := unquoteJSONPathField(content)
		if err != nil || field == "" {
			return jsonStep{}, 0, fmt.Errorf("invalid quoted field name")
		}
		return jsonStep{kind: jsonField, field: field}, end + 1, nil
	}
	index, err := strconv.Atoi(content)
	if err != nil || index < 0 {
		return jsonStep{}, 0, fmt.Errorf("only non-negative array indexes and [*] wildcards are supported")
	}
	return jsonStep{kind: jsonIndex, index: index}, end + 1, nil
}

func unquoteJSONPathField(quoted string) (string, error) {
	if quoted[0] == '\'' {
		quoted = `"` + strings.ReplaceAll(strings.ReplaceAll(quoted[1:len(quoted)-1], `\"`, `"`), `"`, `\"`) + `"`
	}
	var field string
	if err := json.Unmarshal([]byte(quoted), &field); err != nil {
		return "", err
	}
	return field, nil
}

func buildSQLiteJSONPredicate(steps []jsonStep, comparison JSONComparison, want any) (string, []any, bool) {
	if number, ok := want.(json.Number); ok {
		if _, err := strconv.ParseInt(string(number), 10, 64); err != nil {
			return "", nil, true
		}
	}
	args := make([]any, 0, len(steps)+1)
	var walk func(int, string, string) string
	walk = func(position int, parentValue, parentType string) string {
		alias := fmt.Sprintf("jp%d", position)
		step := steps[position]
		conditions := []string{parentType + "=" + quoteSQLLiteral(parentContainerType(step))}
		switch step.kind {
		case jsonField:
			conditions = append(conditions, alias+".key=?")
			args = append(args, step.field)
		case jsonIndex:
			conditions = append(conditions, alias+".key=?")
			args = append(args, step.index)
		case jsonWildcard:
		}
		if position+1 < len(steps) {
			conditions = append(conditions, walk(position+1, alias+".value", alias+".type"))
		} else if comparison == JSONEquals {
			condition, argument, native := sqliteEquality(alias, want)
			if !native {
				return ""
			}
			conditions = append(conditions, condition)
			if argument != nil {
				args = append(args, argument)
			}
		}
		return "EXISTS (SELECT 1 FROM json_each(" + parentValue + ") AS " + alias + " WHERE " + strings.Join(conditions, " AND ") + ")"
	}
	predicate := walk(0, "r.contents", "json_type(r.contents)")
	if predicate == "" {
		return "", nil, true
	}
	return predicate, args, false
}

func parentContainerType(step jsonStep) string {
	if step.kind == jsonField {
		return "object"
	}
	return "array"
}

func sqliteEquality(alias string, want any) (string, any, bool) {
	switch value := want.(type) {
	case nil:
		return alias + ".type='null'", nil, true
	case bool:
		if value {
			return alias + ".type='true'", nil, true
		}
		return alias + ".type='false'", nil, true
	case string:
		return alias + ".type='text' AND " + alias + ".atom=?", value, true
	case json.Number:
		integer, err := strconv.ParseInt(string(value), 10, 64)
		if err != nil {
			return "", nil, false
		}
		return alias + ".type='integer' AND " + alias + ".atom=?", integer, true
	default:
		return "", nil, false
	}
}

func quoteSQLLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

func (filter *compiledJSONFilter) matches(contents []byte) bool {
	if filter == nil {
		return true
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		return false
	}
	selected := []any{root}
	for _, step := range filter.steps {
		next := make([]any, 0)
		for _, candidate := range selected {
			switch step.kind {
			case jsonField:
				object, ok := candidate.(map[string]any)
				if value, exists := object[step.field]; ok && exists {
					next = append(next, value)
				}
			case jsonIndex:
				array, ok := candidate.([]any)
				if ok && step.index < len(array) {
					next = append(next, array[step.index])
				}
			case jsonWildcard:
				if array, ok := candidate.([]any); ok {
					next = append(next, array...)
				}
			}
		}
		selected = next
	}
	if filter.comparison == JSONExists {
		return len(selected) > 0
	}
	for _, selectedValue := range selected {
		if equalJSONScalar(selectedValue, filter.want) {
			return true
		}
	}
	return false
}

func equalJSONScalar(left, right any) bool {
	leftNumber, leftIsNumber := left.(json.Number)
	rightNumber, rightIsNumber := right.(json.Number)
	if leftIsNumber || rightIsNumber {
		if !leftIsNumber || !rightIsNumber {
			return false
		}
		leftRat, leftOK := new(big.Rat).SetString(string(leftNumber))
		rightRat, rightOK := new(big.Rat).SetString(string(rightNumber))
		return leftOK && rightOK && leftRat.Cmp(rightRat) == 0
	}
	return left == right
}
