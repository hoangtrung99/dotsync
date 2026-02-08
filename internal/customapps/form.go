package customapps

import (
	"fmt"
	"strings"

	"dotsync/internal/models"
)

// FormInput contains data entered by user in Add Custom Source flow.
type FormInput struct {
	Mode     string
	Name     string
	Paths    []string
	Category string
	Icon     string
}

// BuildDefinition validates form data and creates an AppDefinition.
func BuildDefinition(in FormInput) (models.AppDefinition, error) {
	mode := strings.ToLower(strings.TrimSpace(in.Mode))
	if mode == "" {
		mode = "folder"
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.AppDefinition{}, fmt.Errorf("name is required")
	}

	paths := make([]string, 0, len(in.Paths))
	for _, p := range in.Paths {
		n := normalizePath(p)
		if n != "" {
			paths = append(paths, n)
		}
	}
	if len(paths) == 0 {
		return models.AppDefinition{}, fmt.Errorf("at least one path is required")
	}

	switch mode {
	case "folder":
		if len(paths) != 1 {
			return models.AppDefinition{}, fmt.Errorf("folder mode requires exactly one path")
		}
	case "app":
		// supports multiple paths
	default:
		return models.AppDefinition{}, fmt.Errorf("invalid mode %q", in.Mode)
	}

	def := models.AppDefinition{
		ID:          slugify(name),
		Name:        name,
		Category:    strings.TrimSpace(in.Category),
		Icon:        strings.TrimSpace(in.Icon),
		ConfigPaths: paths,
	}
	return sanitizeDefinition(def)
}

func slugify(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range s {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "custom"
	}
	return out
}
