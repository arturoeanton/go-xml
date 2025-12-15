package xml

import (
	"fmt"
	"strconv"
	"strings"
)

// ============================================================================
// 5. QUERY & VALIDATION (Sin Cambios Mayores)
// ============================================================================

func QueryAll(data any, path string) ([]any, error) {
	if path == "" {
		return []any{data}, nil
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

			for _, node := range nodesToSearch {
				key, fKey, fVal, idx := parseSegment(segment)

				// ========================================================
				// [INSERTAR AQUÍ] LOGICA SMART #TEXT
				// ========================================================
				// Si piden "#text" y el nodo es un primitivo (string/int),
				// lo agregamos directo y pasamos al siguiente.
				if key == "#text" {
					switch node.(type) {
					case string, int, float64, bool:
						nextCandidates = append(nextCandidates, node)
						continue
					}
				}
				// ========================================================

				if m, ok := node.(map[string]any); ok {
					val, exists := m[key]
					if !exists {
						continue
					}

					if fKey != "" { // Filtro
						if list, ok := val.([]any); ok {
							for _, item := range list {
								if matchFilter(item, fKey, fVal) {
									nextCandidates = append(nextCandidates, item)
								}
							}
						}
					} else if idx >= 0 { // Indice
						if list, ok := val.([]any); ok {
							if idx < len(list) {
								nextCandidates = append(nextCandidates, list[idx])
							}
						}
					} else { // Todo
						if list, ok := val.([]any); ok {
							nextCandidates = append(nextCandidates, list...)
						} else {
							nextCandidates = append(nextCandidates, val)
						}
					}
				}
			}
		}
		if len(nextCandidates) == 0 {
			return nil, nil
		}
		currentCandidates = nextCandidates
	}
	return currentCandidates, nil
}

func parseSegment(seg string) (key, fKey, fVal string, idx int) {
	idx = -1
	key = seg
	if i := strings.Index(seg, "["); i > 0 && strings.HasSuffix(seg, "]") {
		key = seg[:i]
		inside := seg[i+1 : len(seg)-1]
		if strings.Contains(inside, "=") {
			parts := strings.SplitN(inside, "=", 2)
			fKey = parts[0]
			fVal = parts[1]
		} else {
			idx, _ = strconv.Atoi(inside)
		}
	}
	return
}

func matchFilter(item any, k, v string) bool {
	if m, ok := item.(map[string]any); ok {
		if val, exists := m[k]; exists && fmt.Sprintf("%v", val) == v {
			return true
		}
		if val, exists := m["@"+k]; exists && fmt.Sprintf("%v", val) == v {
			return true
		}
	}
	return false
}

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

type Rule struct {
	Path     string
	Required bool
	Type     string
	Min      float64
	Max      float64
	Regex    string
	Enum     []string
}
