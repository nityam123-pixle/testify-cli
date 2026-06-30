package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// extractTSClassBody extracts the body text of "class ClassName {" or "interface InterfaceName {"
func extractTSClassBody(text, className string) string {
	pattern := regexp.MustCompile(`(?s)(?:class|interface)\s+` + regexp.QuoteMeta(className) + `\s*(?:implements\s+[^{]+)?(?:extends\s+[^{]+)?\{(.+?)\n\}`)
	m := pattern.FindStringSubmatch(text)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// parseTSClass parses the lines of a TypeScript class/interface body and extracts the property names
func parseTSClass(classBody string) map[string]interface{} {
	schema := make(map[string]interface{})
	braceDepth := 0
	for _, line := range strings.Split(classBody, "\n") {
		trimmed := strings.TrimSpace(line)
		
		braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
		
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		
		if braceDepth > 0 || (braceDepth == 0 && strings.HasPrefix(trimmed, "}")) {
			continue
		}
		
		// Skip decorators, methods
		if strings.HasPrefix(trimmed, "@") || strings.Contains(trimmed, "(") {
			continue
		}
		// Match property: "name: string;" or "age?: number;" or "readonly id: string;"
		// Strip access modifiers
		trimmed = strings.TrimPrefix(trimmed, "public ")
		trimmed = strings.TrimPrefix(trimmed, "private ")
		trimmed = strings.TrimPrefix(trimmed, "protected ")
		trimmed = strings.TrimPrefix(trimmed, "readonly ")
		
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx <= 0 {
			continue
		}
		
		fieldName := strings.TrimSpace(trimmed[:colonIdx])
		fieldName = strings.TrimSuffix(fieldName, "?") // optional fields
		
		if strings.ContainsAny(fieldName, " ()[]\"'") || fieldName == "" {
			continue
		}
		
		schema[fieldName] = tsFieldDefault(fieldName)
	}
	return schema
}

func resolveTSModel(modelName, routeText, routeFile string) map[string]interface{} {
	// Step 1: same file
	body := extractTSClassBody(routeText, modelName)
	if body != "" {
		fields := parseTSClass(body)
		if len(fields) > 0 {
			return fields
		}
	}

	// Step 2: grep all .ts files
	projectRoot := findProjectRoot(routeFile)
	var result map[string]interface{}
	filepath.Walk(projectRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil || result != nil {
			return nil
		}
		if fi.IsDir() {
			name := fi.Name()
			if name == "node_modules" || name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".ts") || path == routeFile {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		body := extractTSClassBody(string(data), modelName)
		if body != "" {
			fields := parseTSClass(body)
			if len(fields) > 0 {
				result = fields
			}
		}
		return nil
	})
	return result
}
