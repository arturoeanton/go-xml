package xml

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ============================================================================
// QUERY ENGINE (Merged from query.go)
// ============================================================================

// QueryAll searches the data structure for all nodes matching the provided path.
func QueryAll(data any, path string) ([]any, error) {
	if path == "" {
		return []any{data}, nil
	}

	if strings.HasPrefix(path, "//") {
		targetKey := strings.TrimPrefix(path, "//")
		return findAllRecursively(data, targetKey), nil
	}

	segments := strings.Split(path, "/")
	currentCandidates := []any{data}

	for _, segment := range segments {
		if segment == "" {
			continue
		}
		var nextCandidates []any
		for _, candidate := range currentCandidates {
			nodesToSearch := []any{candidate}
			if list, ok := candidate.([]any); ok {
				nodesToSearch = list
			}

			// #count logic
			if segment == "#count" {
				val := 0
				if list, ok := candidate.([]any); ok {
					val = len(list)
				} else if m, ok := candidate.(*OrderedMap); ok {
					val = m.Len()
				} else if m, ok := candidate.(map[string]any); ok {
					val = len(m)
				}
				nextCandidates = append(nextCandidates, val)
				continue
			}

			for _, node := range nodesToSearch {
				key, fParams, idx := parseSegment(segment)

				if key == "#text" {
					switch node.(type) {
					case string, int, float64, bool:
						nextCandidates = append(nextCandidates, node)
						continue
					}
				}

				var valuesToProcess []any

				if m, ok := node.(*OrderedMap); ok {
					if key == "*" {
						m.ForEach(func(k string, v any) bool {
							if !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "#") {
								valuesToProcess = append(valuesToProcess, v)
							}
							return true
						})
					} else if strings.HasPrefix(key, "func:") {
						funcName := strings.TrimPrefix(key, "func:")
						if fn, ok := getQueryFunction(funcName); ok {
							m.ForEach(func(k string, v any) bool {
								if !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "#") {
									if fn(k) {
										valuesToProcess = append(valuesToProcess, v)
									}
								}
								return true
							})
						}
					} else {
						if val := m.Get(key); val != nil {
							valuesToProcess = append(valuesToProcess, val)
						}
					}
				} else if m, ok := node.(map[string]any); ok {
					if key == "*" {
						var keys []string
						for k := range m {
							if !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "#") {
								keys = append(keys, k)
							}
						}
						sort.Strings(keys)
						for _, k := range keys {
							valuesToProcess = append(valuesToProcess, m[k])
						}
					} else if strings.HasPrefix(key, "func:") {
						funcName := strings.TrimPrefix(key, "func:")
						if fn, ok := getQueryFunction(funcName); ok {
							var keys []string
							for k := range m {
								if !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "#") {
									if fn(k) {
										keys = append(keys, k)
									}
								}
							}
							sort.Strings(keys)
							for _, k := range keys {
								valuesToProcess = append(valuesToProcess, m[k])
							}
						}
					} else {
						if val, exists := m[key]; exists {
							valuesToProcess = append(valuesToProcess, val)
						}
					}
				}

				for _, val := range valuesToProcess {
					if fParams != nil {
						if list, ok := val.([]any); ok {
							for _, item := range list {
								if matchFilter(item, fParams) {
									nextCandidates = append(nextCandidates, item)
								}
							}
						} else {
							if matchFilter(val, fParams) {
								nextCandidates = append(nextCandidates, val)
							}
						}
					} else if idx >= 0 {
						if list, ok := val.([]any); ok {
							if idx < len(list) {
								nextCandidates = append(nextCandidates, list[idx])
							}
						}
					} else {
						nextCandidates = append(nextCandidates, val)
					}
				}
			}
		}
		if len(nextCandidates) == 0 {
			return nil, nil // Not found
		}
		currentCandidates = nextCandidates
	}
	return currentCandidates, nil
}

type filterParams struct {
	Key    string
	Op     string
	Val    string
	IsFunc bool
}

func parseSegment(seg string) (key string, fp *filterParams, idx int) {
	idx = -1
	key = seg
	if i := strings.Index(seg, "["); i > 0 && strings.HasSuffix(seg, "]") {
		key = seg[:i]
		inside := seg[i+1 : len(seg)-1]

		if strings.Contains(inside, "(") && strings.Contains(inside, ")") {
			pIndex := strings.Index(inside, "(")
			funcName := strings.TrimSpace(inside[:pIndex])
			argsStr := inside[pIndex+1 : len(inside)-1]
			args := strings.Split(argsStr, ",")
			if len(args) == 2 {
				fKey := strings.TrimSpace(args[0])
				fVal := strings.TrimSpace(args[1])
				fVal = strings.Trim(fVal, "'\"")
				return key, &filterParams{Key: fKey, Op: funcName, Val: fVal, IsFunc: true}, -1
			}
		}

		ops := []string{"!=", ">=", "<=", "=", ">", "<"}
		for _, op := range ops {
			if strings.Contains(inside, op) {
				parts := strings.SplitN(inside, op, 2)
				fKey := strings.TrimSpace(parts[0])
				fVal := strings.TrimSpace(parts[1])
				fVal = strings.Trim(fVal, "'\"")
				return key, &filterParams{Key: fKey, Op: op, Val: fVal, IsFunc: false}, -1
			}
		}

		if val, err := strconv.Atoi(inside); err == nil {
			idx = val
		}
	}
	return
}

func matchFilter(item any, fp *filterParams) bool {
	var actual any
	found := false

	if m, ok := item.(*OrderedMap); ok {
		if v := m.Get(fp.Key); v != nil {
			actual = v
			found = true
		} else if v := m.Get("@" + fp.Key); v != nil {
			actual = v
			found = true
		}
	} else if m, ok := item.(map[string]any); ok {
		if v, exists := m[fp.Key]; exists {
			actual = v
			found = true
		} else if v, exists := m["@"+fp.Key]; exists {
			actual = v
			found = true
		}
	}

	if !found {
		return false
	}

	actualStr := fmt.Sprintf("%v", actual)

	if fp.IsFunc {
		switch fp.Op {
		case "contains":
			return strings.Contains(actualStr, fp.Val)
		case "starts-with":
			return strings.HasPrefix(actualStr, fp.Val)
		}
		return false
	}

	switch fp.Op {
	case "=":
		return actualStr == fp.Val
	case "!=":
		return actualStr != fp.Val
	case ">", "<", ">=", "<=":
		numV, errV := strconv.ParseFloat(actualStr, 64)
		targetV, errT := strconv.ParseFloat(fp.Val, 64)
		if errV != nil || errT != nil {
			return false
		}
		switch fp.Op {
		case ">":
			return numV > targetV
		case "<":
			return numV < targetV
		case ">=":
			return numV >= targetV
		case "<=":
			return numV <= targetV
		}
	}
	return false
}

func findAllRecursively(data any, targetKey string) []any {
	var results []any
	var traverse func(node any)
	traverse = func(node any) {
		if m, ok := node.(*OrderedMap); ok {
			if val := m.Get(targetKey); val != nil {
				results = append(results, val)
			}
			m.ForEach(func(k string, v any) bool {
				traverse(v)
				return true
			})
		} else if m, ok := node.(map[string]any); ok {
			if val, exists := m[targetKey]; exists {
				results = append(results, val)
			}
			var keys []string
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				traverse(m[k])
			}
		} else if list, ok := node.([]any); ok {
			for _, item := range list {
				traverse(item)
			}
		}
	}
	traverse(data)
	return results
}

// Query is a convenience wrapper around QueryAll that returns the first matching result.
func Query(data any, path string) (any, error) {
	res, err := QueryAll(data, path)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return res[0], nil
}

// Get realiza una Query y retorna el valor tipado T.
func Get[T any](data any, path string) (T, error) {
	var zero T
	val, err := Query(data, path)
	if err != nil {
		return zero, err
	}

	if v, ok := val.(T); ok {
		return v, nil
	}

	switch any(zero).(type) {
	case string:
		return any(fmt.Sprintf("%v", val)).(T), nil
	case int:
		str := fmt.Sprintf("%v", val)
		if i, err := strconv.Atoi(str); err == nil {
			return any(i).(T), nil
		}
	}

	return zero, fmt.Errorf("value at %s is %T, expected %T", path, val, zero)
}

// Rule defines a validation constraint for the Validate engine.
type Rule struct {
	Path     string
	Required bool
	Type     string
	Min      float64
	Max      float64
	Regex    string
	Enum     []string
}
