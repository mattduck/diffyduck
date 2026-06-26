package rpconfig

import "fmt"

// Built-in rule codes for annotation-quality checks.
const (
	CodeRPTSyntax       = "rpt-syntax"
	CodeRPTUnknownScope = "rpt-unknown-scope"
	CodeRPTTypeMismatch = "rpt-type-mismatch"
)

// BuiltinRules returns the hardcoded annotation-quality rules. They are
// disabled by default and must be enabled via -select or -extend-select.
func BuiltinRules() []Rule {
	return []Rule{
		{
			Code:        CodeRPTSyntax,
			Title:       "Malformed RPT annotation",
			Description: "RPT annotation does not match the expected format (RPT type(scope): message or RPT type: message). The annotation is ignored by rpt check.",
		},
		{
			Code:        CodeRPTUnknownScope,
			Title:       "Unknown RPT rule scope",
			Description: "RPT annotation references a rule scope that is not defined in revparrot.toml.",
		},
		{
			Code:        CodeRPTTypeMismatch,
			Title:       "RPT annotation type mismatch",
			Description: "RPT annotation type does not match the type declared for the rule in revparrot.toml.",
		},
	}
}

// IsBuiltinCode reports whether code is one of the hardcoded built-in rules.
func IsBuiltinCode(code string) bool {
	switch code {
	case CodeRPTSyntax, CodeRPTUnknownScope, CodeRPTTypeMismatch:
		return true
	}
	return false
}

// AllRuleByCode looks up a rule in cfg (if non-nil) first, then in the
// built-in rules. Returns false if not found in either.
func AllRuleByCode(cfg *Config, code string) (Rule, bool) {
	if cfg != nil {
		if r, ok := cfg.RuleByCode(code); ok {
			return r, true
		}
	}
	for _, r := range BuiltinRules() {
		if r.Code == code {
			return r, true
		}
	}
	return Rule{}, false
}

// ResolveRuleset builds the active rule set from config and selection flags.
//
//   - If selectList is non-empty, those codes replace the default config active
//     rules. Each code must exist in cfg or in BuiltinRules.
//   - extendList codes are added to the current active set (deduped). Each code
//     must exist in cfg or in BuiltinRules.
//   - If both lists are empty and cfg is non-nil, returns cfg.ActiveRules().
//   - If both lists are empty and cfg is nil, returns nil (no filter: all
//     violations pass through; no built-ins are activated).
func ResolveRuleset(cfg *Config, selectList, extendList []string) ([]Rule, error) {
	lookup := func(code string) (Rule, bool) {
		return AllRuleByCode(cfg, code)
	}

	var active []Rule

	if len(selectList) > 0 {
		for _, code := range selectList {
			r, ok := lookup(code)
			if !ok {
				return nil, fmt.Errorf("unknown rule code: %q", code)
			}
			active = append(active, r)
		}
	} else if cfg != nil {
		active = cfg.ActiveRules()
	}

	for _, code := range extendList {
		r, ok := lookup(code)
		if !ok {
			return nil, fmt.Errorf("unknown rule code: %q", code)
		}
		dup := false
		for _, existing := range active {
			if existing.Code == code {
				dup = true
				break
			}
		}
		if !dup {
			active = append(active, r)
		}
	}

	return active, nil
}
