package xml

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ============================================================================
// 5. QUERY ENGINE
// ============================================================================

// QueryAll searches the data structure for all nodes matching the provided path.
// It returns a slice of matches found.
//
// Path Syntax:
//   - Deep Navigation: "library/section/book" (Traverse nested maps)
//   - Array Indexing:  "users/user[0]" (Access specific index)
//   - Attribute/Value Filtering: "users/user[id=5]" or "book[@lang=en]"
//   - Text Extraction: "book/title/#text" (Explicit text node access)
//
// Note: If the path targets a list directly (e.g., "tags"), QueryAll returns
// the list itself as a single result in the slice, rather than flattening it.
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
			// Normalize candidate to a list for iteration.
			// This handles the automatic "flattening" required for deep search
			// when the previous result was a list of objects.
			nodesToSearch := []any{candidate}
			if list, ok := candidate.([]any); ok {
				nodesToSearch = list
			}

			for _, node := range nodesToSearch {
				key, fKey, fVal, idx := parseSegment(segment)

				// ========================================================
				// SMART #TEXT LOGIC
				// ========================================================
				// If the user requests "#text" and the node is a primitive
				// (string/int/float/bool) resulting from parser simplification,
				// we return the value directly.
				if key == "#text" {
					switch node.(type) {
					case string, int, float64, bool:
						nextCandidates = append(nextCandidates, node)
						continue
					}
				}
				// ========================================================

				if m, ok := node.(map[string]any); ok {
					var valuesToProcess []any

					if key == "*" {
						// Wildcard Strategy: Match all child nodes (excluding attributes/metadata).
						// We sort keys to ensure deterministic results.
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
						// Custom Function Strategy
						funcName := strings.TrimPrefix(key, "func:")
						if fn, ok := getQueryFunction(funcName); ok {
							var keys []string
							for k := range m {
								// Standard behavior: skip attributes/metadata unless user explicitly asks?
								// Let's pass CLEAN keys to the function? Or all keys?
								// Ideally, we follow wildcard logic: filter out @ and # automatically first,
								// THEN apply the custom function to the "content" node names.
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
						// Direct Key Strategy
						if val, exists := m[key]; exists {
							valuesToProcess = append(valuesToProcess, val)
						}
					}

					for _, val := range valuesToProcess {
						if fKey != "" {
							// Filter Strategy: [key=value]
							// Iterates over the list and selects items matching criteria.
							if list, ok := val.([]any); ok {
								for _, item := range list {
									if matchFilter(item, fKey, fVal) {
										nextCandidates = append(nextCandidates, item)
									}
								}
							}
						} else if idx >= 0 {
							// Index Strategy: [i]
							// Selects a specific element from the list.
							if list, ok := val.([]any); ok {
								if idx < len(list) {
									nextCandidates = append(nextCandidates, list[idx])
								}
							}
						} else {
							// Select All Strategy
							// IMPORTANT: We append the value 'as is'.
							// If 'val' is a list (e.g., tags: ["a", "b"]), we append the list itself.
							// We do NOT flatten here because QueryAll results should represent
							// the distinct nodes found at this path level.
							nextCandidates = append(nextCandidates, val)
						}
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

// parseSegment parses a path segment to extract the key, filter parameters, or index.
// It handles syntax like "user", "user[0]", and "user[id=1]".
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

// matchFilter checks if an item satisfies the "key=value" condition.
// It checks both direct keys and attribute keys (prefixed with "@").
func matchFilter(item any, k, v string) bool {
	if m, ok := item.(map[string]any); ok {
		// Check direct child value
		if val, exists := m[k]; exists && fmt.Sprintf("%v", val) == v {
			return true
		}
		// Check attribute value
		if val, exists := m["@"+k]; exists && fmt.Sprintf("%v", val) == v {
			return true
		}
	}
	return false
}

// Query is a convenience wrapper around QueryAll that returns the first matching result.
// It returns an error if no matching node is found.
// This is useful when you expect a single value or only care about the first match.
func Query(data any, path string) (any, error) {
	res, err := QueryAll(data, path)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("not found")
	}
	// Since QueryAll preserves lists as single items, res[0] is the correct result.
	return res[0], nil
}

// Rule defines a validation constraint for the Validate engine.
// It is used to enforce schema-like requirements on dynamic maps.
type Rule struct {
	// Path to the element to validate (e.g., "server/port").
	Path string

	// Required enforces that the path must exist.
	Required bool

	// Type enforces the data type ("int", "float", "string", "array", "bool").
	Type string

	// Min enforces a minimum numeric value.
	Min float64

	// Max enforces a maximum numeric value.
	Max float64

	// Regex enforces a string pattern match.
	Regex string

	// Enum enforces that the value must be one of the provided strings.
	Enum []string
}
