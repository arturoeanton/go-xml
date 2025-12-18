package xml

import (
	"strings"
	"sync"
)

// QueryFunction defines the signature for custom key filter functions.
// It receives a map key (string) and returns true if the key should be traversed.
type QueryFunction func(key string) bool

var (
	queryFunctions   = make(map[string]QueryFunction)
	queryFunctionsMu sync.RWMutex
)

// RegisterQueryFunction registers a custom function for use in Query paths.
// The name used here must match the segment in the path after "func:".
// Example: RegisterQueryFunction("startsWithBox", ...) -> Path "items/func:startsWithBox/sku"
func RegisterQueryFunction(name string, fn QueryFunction) {
	queryFunctionsMu.Lock()
	defer queryFunctionsMu.Unlock()
	queryFunctions[name] = fn
}

// getQueryFunction retrieves a registered function by name.
func getQueryFunction(name string) (QueryFunction, bool) {
	queryFunctionsMu.RLock()
	defer queryFunctionsMu.RUnlock()
	fn, ok := queryFunctions[name]
	return fn, ok
}

func init() {

	// 1. isNumeric: Checks if the key contains only digits.
	// Usage: path/to/func:isNumeric/child
	RegisterQueryFunction("isNumeric", func(key string) bool {
		for _, r := range key {
			if r < '0' || r > '9' {
				return false
			}
		}
		return len(key) > 0
	})

	// 2. isAlpha: Checks if the key contains only letters (a-z, A-Z).
	// Usage: path/to/func:isAlpha/child
	RegisterQueryFunction("isAlpha", func(key string) bool {
		for _, r := range key {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
				return false
			}
		}
		return len(key) > 0
	})

	// 3. isAlphanumeric: Checks if the key contains only letters and digits.
	// Usage: path/to/func:isAlphanumeric/child
	RegisterQueryFunction("isAlphanumeric", func(key string) bool {
		for _, r := range key {
			isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
			isDigit := (r >= '0' && r <= '9')
			if !isLetter && !isDigit {
				return false
			}
		}
		return len(key) > 0
	})

	// 4. isLower: Checks if the key is all lower case.
	// Usage: path/to/func:isLower/child
	RegisterQueryFunction("isLower", func(key string) bool {
		return key == strings.ToLower(key) && len(key) > 0
	})

	// 5. isUpper: Checks if the key is all upper case.
	// Usage: path/to/func:isUpper/child
	RegisterQueryFunction("isUpper", func(key string) bool {
		return key == strings.ToUpper(key) && len(key) > 0
	})

	// 6. hasUnderscore: Checks if the key contains at least one underscore.
	// Usage: path/to/func:hasUnderscore/child
	RegisterQueryFunction("hasUnderscore", func(key string) bool {
		return strings.Contains(key, "_")
	})

	// 7. hasHyphen: Checks if the key contains at least one hyphen.
	// Usage: path/to/func:hasHyphen/child
	RegisterQueryFunction("hasHyphen", func(key string) bool {
		return strings.Contains(key, "-")
	})

	// 8. isSnakeCase: Checks if key matches snake_case pattern (lowercase, underscores, no leading/trailing underscores).
	// Usage: path/to/func:isSnakeCase/child
	RegisterQueryFunction("isSnakeCase", func(key string) bool {
		if len(key) == 0 {
			return false
		}
		if key != strings.ToLower(key) || strings.Contains(key, "-") {
			return false
		}
		// Typically snake_case has underscores, but could be single word too.
		// Strict snake_case: must have at least one underscore?
		// Let's assume standard programming convention: usually implies words separated by _.
		// But "variable" is considered valid snake case in some validators.
		// Let's enforce structural validity: letters, digits, underscores.
		for _, r := range key {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
		return true
	})

	// 9. isKebabCase: Checks if key matches kebab-case pattern (lowercase, hyphens).
	// Usage: path/to/func:isKebabCase/child
	RegisterQueryFunction("isKebabCase", func(key string) bool {
		if len(key) == 0 {
			return false
		}
		if key != strings.ToLower(key) || strings.Contains(key, "_") {
			return false
		}
		for _, r := range key {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
		return true
	})

	// 10. isCamelCase: Checks if key matches camelCase pattern (starts lower, mixed case).
	// Usage: path/to/func:isCamelCase/child
	RegisterQueryFunction("isCamelCase", func(key string) bool {
		if len(key) == 0 {
			return false
		}
		first := rune(key[0])
		if first < 'a' || first > 'z' {
			return false // Must start with lower
		}
		// Must contain no separators
		if strings.ContainsAny(key, "_-") {
			return false
		}
		return true
	})

	// 11. isPascalCase: Checks if key matches PascalCase pattern (starts upper, mixed case).
	// Usage: path/to/func:isPascalCase/child
	RegisterQueryFunction("isPascalCase", func(key string) bool {
		if len(key) == 0 {
			return false
		}
		first := rune(key[0])
		if first < 'A' || first > 'Z' {
			return false // Must start with upper
		}
		if strings.ContainsAny(key, "_-") {
			return false
		}
		return true
	})

	// 12. startsWithUnderscore: Checks for keys starting with _.
	// Usage: path/to/func:startsWithUnderscore/child
	RegisterQueryFunction("startsWithUnderscore", func(key string) bool {
		return strings.HasPrefix(key, "_")
	})

	// 13. startsWithDot: Checks for keys starting with . (often hidden files).
	// Usage: path/to/func:startsWithDot/child
	RegisterQueryFunction("startsWithDot", func(key string) bool {
		return strings.HasPrefix(key, ".")
	})

	// 14. hasDigits: Checks if the key contains any digit.
	// Usage: path/to/func:hasDigits/child
	RegisterQueryFunction("hasDigits", func(key string) bool {
		return strings.ContainsAny(key, "0123456789")
	})

	// 15. isUUID: Rough check if key looks like a UUID (36 chars, contains hyphens).
	// Usage: path/to/func:isUUID/child
	RegisterQueryFunction("isUUID", func(key string) bool {
		// e.g., 550e8400-e29b-41d4-a716-446655440000
		if len(key) != 36 {
			return false
		}
		return strings.Count(key, "-") == 4
	})
}
