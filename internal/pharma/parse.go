package pharma

import "strings"

// PharmClass represents a parsed pharmacological class with name and type.
type PharmClass struct {
	Name string
	Type string
}

// ParsePharmClass parses a single FDA pharm_class string like
// "HMG-CoA Reductase Inhibitor [EPC]" into name and type.
// If no bracket suffix is found, name is the full string and type is empty.
func ParsePharmClass(s string) PharmClass {
	idx := strings.LastIndex(s, "[")
	if idx < 0 {
		return PharmClass{Name: s, Type: ""}
	}
	name := strings.TrimSpace(s[:idx])
	typ := strings.TrimSuffix(s[idx+1:], "]")
	return PharmClass{Name: name, Type: typ}
}

// ParsePharmClasses parses a slice of FDA pharm_class strings.
// Returns an empty (non-nil) slice for nil input.
func ParsePharmClasses(ss []string) []PharmClass {
	result := make([]PharmClass, 0, len(ss))
	for _, s := range ss {
		result = append(result, ParsePharmClass(s))
	}
	return result
}
