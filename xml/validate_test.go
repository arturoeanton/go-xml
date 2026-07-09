package xml

import "testing"

// ============================================================================
// VALIDATION TESTS
// ============================================================================
func TestValidate_Rules(t *testing.T) {
	data := map[string]any{
		"user": map[string]any{
			"name":  "Alice",
			"age":   "25", // String that must pass as a number
			"role":  "admin",
			"email": "alice@example.com",
			"tags":  []any{"dev", "ops"},
		},
	}

	tests := []struct {
		name      string
		rules     []Rule
		wantError bool
	}{
		{
			name: "Success All Rules",
			rules: []Rule{
				{Path: "user/name", Required: true, Type: "string"},
				{Path: "user/age", Type: "int", Min: 18, Max: 99},
				{Path: "user/role", Enum: []string{"admin", "guest"}, Type: "string"},
				{Path: "user/tags", Type: "array"},
				{Path: "user/email", Regex: `.+@.+\..+`, Type: "string"},
			},
			wantError: false,
		},
		{
			name: "Fail Required",
			rules: []Rule{
				{Path: "user/phone", Required: true},
			},
			wantError: true,
		},
		{
			name: "Fail Min Value",
			rules: []Rule{
				// FIX: Added Type: "int" to trigger the numeric logic
				{Path: "user/age", Min: 30, Type: "int"},
			},
			wantError: true,
		},
		{
			name: "Fail Enum",
			rules: []Rule{
				// FIX: Added Type: "string" to trigger the Enum logic
				{Path: "user/role", Enum: []string{"guest", "support"}, Type: "string"},
			},
			wantError: true,
		},
		{
			name: "Fail Regex",
			rules: []Rule{
				// FIX: Added Type: "string" to trigger the Regex logic
				{Path: "user/name", Regex: `^[0-9]+$`, Type: "string"},
			},
			wantError: true,
		},
		{
			name: "Fail Type Array",
			rules: []Rule{
				{Path: "user/name", Type: "array"},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(data, tt.rules)
			if tt.wantError && len(errs) == 0 {
				t.Errorf("Expected validation errors, got none")
			}
			if !tt.wantError && len(errs) > 0 {
				t.Errorf("Unexpected validation errors: %v", errs)
			}
		})
	}
}
