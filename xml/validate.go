package xml

import (
	"fmt"
	"regexp"
	"strconv"
)

// ============================================================================
// VALIDATION ENGINE
// ============================================================================
// (Mantener el código de Validate igual que tenías, está correcto)
func Validate(data any, rules []Rule) []string {
	var errs []string
	for _, r := range rules {
		val, err := Query(data, r.Path)
		if err != nil {
			if r.Required {
				errs = append(errs, "Missing: "+r.Path)
			}
			continue
		}
		var floatVal float64
		var strVal string
		isNum := false
		isStr := false
		switch r.Type {
		case "array":
			if _, ok := val.([]any); !ok {
				errs = append(errs, fmt.Sprintf("%s must be an array", r.Path))
			}
		case "int", "float":
			if v, ok := asFloat(val); ok {
				floatVal = v
				isNum = true
			} else {
				errs = append(errs, fmt.Sprintf("%s must be numeric", r.Path))
			}
		case "string":
			strVal = fmt.Sprintf("%v", val)
			isStr = true
		}
		if isNum {
			if r.Min != 0 && floatVal < r.Min {
				errs = append(errs, fmt.Sprintf("%s value %.2f is less than minimum %.2f", r.Path, floatVal, r.Min))
			}
			if r.Max != 0 && floatVal > r.Max {
				errs = append(errs, fmt.Sprintf("%s value %.2f is greater than maximum %.2f", r.Path, floatVal, r.Max))
			}
		}
		if isStr {
			if r.Regex != "" {
				matched, _ := regexp.MatchString(r.Regex, strVal)
				if !matched {
					errs = append(errs, fmt.Sprintf("%s invalid format (Regex)", r.Path))
				}
			}
			if len(r.Enum) > 0 {
				found := false
				for _, allowed := range r.Enum {
					if strVal == allowed {
						found = true
						break
					}
				}
				if !found {
					errs = append(errs, fmt.Sprintf("%s invalid value. Allowed: %v", r.Path, r.Enum))
				}
			}
		}
	}
	return errs
}

func asFloat(v any) (float64, bool) {
	switch i := v.(type) {
	case int:
		return float64(i), true
	case float64:
		return i, true
	case string:
		if f, err := strconv.ParseFloat(i, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
