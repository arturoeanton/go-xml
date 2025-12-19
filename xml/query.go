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
// QueryAll searches the data structure for all nodes matching the provided path.
// It returns a slice of matches found.
//
// Path Syntax:
//   - Deep Navigation: "library/section/book"
//   - Deep Search:     "//error" (Find "error" nodes anywhere)
//   - Array Indexing:  "users/user[0]"
//   - Filter Logic:    "book[price>10]" or "user[role='admin']" or "user[id!=5]"
//   - Filter Funcs:    "book[contains(title, 'Go')]" or "user[starts-with(name, 'A')]"
//   - Wildcards:       "items/*/sku"
//   - Custom Funcs:    "items/func:isNumeric/id"
//   - Meta-Properties: "items/#count" (Returns number of children)
//   - Text Extraction: "book/title/#text"
//
// QueryAll searches the data structure for all nodes matching the provided path.
// It returns a slice of matches found.
//
// Path Syntax:
//   - Deep Navigation: "library/section/book"
//   - Deep Search:     "//error" (Find "error" nodes anywhere)
//   - Array Indexing:  "users/user[0]"
//   - Filter Logic:    "book[price>10]" or "user[role='admin']" or "user[id!=5]"
//   - Filter Funcs:    "book[contains(title, 'Go')]" or "user[starts-with(name, 'A')]"
//   - Wildcards:       "items/*/sku"
//   - Custom Funcs:    "items/func:isNumeric/id"
//   - Meta-Properties: "items/#count" (Returns number of children)
//   - Text Extraction: "book/title/#text"
func QueryAll(data any, path string) ([]any, error) {
	if path == "" {
		return []any{data}, nil
	}

	// ========================================================
	// DEEP SEARCH LOGIC (//)
	// ========================================================
	if strings.HasPrefix(path, "//") {
		targetKey := strings.TrimPrefix(path, "//")
		if idx := strings.Index(targetKey, "/"); idx != -1 {
			// e.g. //section/book -> find section recursively, then navigate /book
			// This recursion is complex. Let's do a simple recursive finder for the *first* name.
		}
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
			// Normalize candidate to a list for iteration.
			nodesToSearch := []any{candidate}
			if list, ok := candidate.([]any); ok {
				nodesToSearch = list
			}

			// ========================================================
			// #COUNT LOGIC
			// ========================================================
			if segment == "#count" {
				val := 0
				if list, ok := candidate.([]any); ok {
					val = len(list)
				} else if m, ok := candidate.(*OrderedMap); ok {
					val = m.Len()
				} else if m, ok := candidate.(map[string]any); ok { // Fallback for legacy maps
					val = len(m)
				}
				nextCandidates = append(nextCandidates, val)
				continue
			}

			for _, node := range nodesToSearch {
				key, fParams, idx := parseSegment(segment)

				// ========================================================
				// SMART #TEXT LOGIC
				// ========================================================
				if key == "#text" {
					switch node.(type) {
					case string, int, float64, bool:
						nextCandidates = append(nextCandidates, node)
						continue
					}
				}

				// Check for OrderedMap (new) or map[string]any (legacy support)
				var valuesToProcess []any

				if m, ok := node.(*OrderedMap); ok {
					if key == "*" {
						// Wildcard Strategy
						m.ForEach(func(k string, v any) bool {
							if !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "#") {
								valuesToProcess = append(valuesToProcess, v)
							}
							return true
						})
					} else if strings.HasPrefix(key, "func:") {
						// Custom Function Strategy
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
						// Direct Key Strategy
						if val := m.Get(key); val != nil {
							valuesToProcess = append(valuesToProcess, val)
						}
					}
				} else if m, ok := node.(map[string]any); ok {
					if key == "*" {
						// Wildcard Strategy
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
				}

				// Process found values (filter/index)
				for _, val := range valuesToProcess {
					if fParams != nil {
						// Filter Strategy with Enhanced Operators
						if list, ok := val.([]any); ok {
							for _, item := range list {
								if matchFilter(item, fParams) {
									nextCandidates = append(nextCandidates, item)
								}
							}
						} else {
							// If it's a single item (object), check filter on itself?
							// Usually filters apply to lists: users/user[id=1]
							// But if 'user' is single object, we can check it too.
							if matchFilter(val, fParams) {
								nextCandidates = append(nextCandidates, val)
							}
						}
					} else if idx >= 0 {
						// Index Strategy
						if list, ok := val.([]any); ok {
							if idx < len(list) {
								nextCandidates = append(nextCandidates, list[idx])
							}
						}
					} else {
						// Select All Strategy
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

// filterParams holds the parsed filter conditions
type filterParams struct {
	Key    string
	Op     string // =, !=, >, <, contains, starts-with
	Val    string
	IsFunc bool // true if Op is contains/starts-with
}

// parseSegment parses a path segment including advanced filters.
// Supports: item[id=5], item[price>10], item[contains(name, 'John')]
func parseSegment(seg string) (key string, fp *filterParams, idx int) {
	idx = -1
	key = seg
	if i := strings.Index(seg, "["); i > 0 && strings.HasSuffix(seg, "]") {
		key = seg[:i]
		inside := seg[i+1 : len(seg)-1]

		// 1. Check for Functions: contains(k, 'v')
		if strings.Contains(inside, "(") && strings.Contains(inside, ")") {
			// Extremely naive parsing for "contains(k, 'v')"
			// Expected format: funcName(key, 'val')
			pIndex := strings.Index(inside, "(")
			funcName := strings.TrimSpace(inside[:pIndex])
			argsStr := inside[pIndex+1 : len(inside)-1]
			args := strings.Split(argsStr, ",")
			if len(args) == 2 {
				fKey := strings.TrimSpace(args[0])
				fVal := strings.TrimSpace(args[1])
				// remove quotes from val if present
				fVal = strings.Trim(fVal, "'\"")
				return key, &filterParams{Key: fKey, Op: funcName, Val: fVal, IsFunc: true}, -1
			}
		}

		// 2. Check for Operators
		// Order matters: >=, <=, != should be checked before >, <, =
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

		// 3. Index fallback
		if val, err := strconv.Atoi(inside); err == nil {
			idx = val
		}
	}
	return
}

// matchFilter checks if an item satisfies the filter condition.
func matchFilter(item any, fp *filterParams) bool {
	var actual any
	found := false

	// Resolve actual value from map or OrderedMap
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

	// Functions
	if fp.IsFunc {
		switch fp.Op {
		case "contains":
			return strings.Contains(actualStr, fp.Val)
		case "starts-with":
			return strings.HasPrefix(actualStr, fp.Val)
		}
		return false
	}

	// Operators
	switch fp.Op {
	case "=":
		return actualStr == fp.Val
	case "!=":
		return actualStr != fp.Val
	case ">", "<", ">=", "<=":
		// Numeric comparison
		numV, errV := strconv.ParseFloat(actualStr, 64)
		targetV, errT := strconv.ParseFloat(fp.Val, 64)
		if errV != nil || errT != nil {
			return false // Cannot compare non-numerics
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

// findAllRecursively traverses the data structure to find all nodes matching targetKey.
// Implements //Node logic.
func findAllRecursively(data any, targetKey string) []any {
	var results []any

	// Helper to check deeper
	var traverse func(node any)
	traverse = func(node any) {
		if m, ok := node.(*OrderedMap); ok {
			// Check direct match
			if val := m.Get(targetKey); val != nil {
				results = append(results, val)
			}
			// Traverse children (Ordered)
			m.ForEach(func(k string, v any) bool {
				traverse(v)
				return true
			})
		} else if m, ok := node.(map[string]any); ok {
			// Check direct match
			if val, exists := m[targetKey]; exists {
				results = append(results, val)
			}
			// Traverse children (Sorted Keys for stability)
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
