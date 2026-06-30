package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nityam123-pixle/testify-cli/internal/detector"
	"github.com/nityam123-pixle/testify-cli/internal/executor"
	"github.com/nityam123-pixle/testify-cli/internal/scanner"
)

type requestEditorModel struct {
	route         scanner.Route
	info          detector.StackInfo
	authInput     textinput.Model
	bodyInput     textarea.Model
	focusIndex    int // 0 = Auth, 1 = Body
	isEditingAuth bool
	submitted     bool
	canceled      bool
	hasBody       bool
	statusMessage string
}

type clearStatusMsg struct{}

// skipParamNames are FastAPI function parameters that are NOT body models.
var skipParamNames = map[string]bool{
	"db": true, "session": true, "current_user": true, "request": true,
	"response": true, "background_tasks": true,
}

// skipParamTypes are FastAPI parameter type annotations that are not body models.
var skipParamTypeKeywords = []string{
	"Depends", "Session", "Optional", "Request", "Response",
	"BackgroundTasks", "str", "int", "bool", "float", "Query", "Path", "Header", "Cookie",
}

// pyTypeDefault maps Python type annotations to their JSON default value.
func pyTypeDefault(typ string) interface{} {
	typ = strings.TrimSpace(typ)
	// Strip Optional[...] wrapper — recurse into inner type
	if strings.HasPrefix(typ, "Optional[") && strings.HasSuffix(typ, "]") {
		return pyTypeDefault(typ[9 : len(typ)-1])
	}
	low := strings.ToLower(typ)
	if strings.Contains(low, "int") || strings.Contains(low, "float") || strings.Contains(low, "decimal") {
		return 0
	}
	if strings.Contains(low, "bool") {
		return false
	}
	if strings.Contains(low, "list") || strings.Contains(low, "set") || strings.HasPrefix(low, "list[") || strings.HasPrefix(low, "set[") {
		return []interface{}{}
	}
	if strings.Contains(low, "dict") || strings.HasPrefix(low, "dict[") {
		return map[string]interface{}{}
	}
	return ""
}

// findProjectRoot walks up from a file path until it finds a project root marker.
func findProjectRoot(startPath string) string {
	dir := filepath.Dir(startPath)
	for i := 0; i < 10; i++ {
		for _, marker := range []string{"requirements.txt", "package.json", "go.mod", "pyproject.toml"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Dir(startPath)
}

// parsePydanticClass reads the class body after "class Name(BaseModel):" and returns a field map.
func parsePydanticClass(classBody string) map[string]interface{} {
	schema := make(map[string]interface{})
	for _, line := range strings.Split(classBody, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Stop when we hit model_config block, class Config, validators, or methods
		if strings.HasPrefix(trimmed, "model_config") ||
			strings.HasPrefix(trimmed, "class Config") ||
			strings.HasPrefix(trimmed, "class ") ||
			strings.HasPrefix(trimmed, "@") ||
			strings.HasPrefix(trimmed, "def ") {
			break
		}
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx <= 0 {
			continue
		}
		fieldName := strings.TrimSpace(trimmed[:colonIdx])
		// Skip quoted keys (from dict literals) or names with special chars
		if strings.ContainsAny(fieldName, " ()[]\"'") || fieldName == "" {
			continue
		}
		rest := strings.TrimSpace(trimmed[colonIdx+1:])
		typeStr := rest
		if eqIdx := strings.Index(rest, "="); eqIdx > 0 {
			beforeEq := rest[:eqIdx]
			if !strings.ContainsAny(beforeEq, "([") {
				typeStr = strings.TrimSpace(beforeEq)
			}
		}
		// When Field(...) is used, type is on the same line before "= Field("
		if strings.HasPrefix(rest, "Field(") || strings.HasPrefix(rest, "field(") {
			typeStr = "str"
		}
		schema[fieldName] = pyTypeDefault(typeStr)
	}
	return schema
}

// extractClassBody extracts the body text of "class ClassName(BaseModel):" from file text.
func extractClassBody(text, className string) string {
	// Match "class ClassName(..." on a line
	pattern := regexp.MustCompile(`(?m)^class\s+` + regexp.QuoteMeta(className) + `\s*\([^)]*\)\s*:`)
	loc := pattern.FindStringIndex(text)
	if loc == nil {
		return ""
	}
	body := text[loc[1]:]
	// Collect lines that are indented (the class body)
	var bodyLines []string
	for _, line := range strings.Split(body, "\n") {
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			bodyLines = append(bodyLines, line)
		} else {
			break // dedented line = end of class
		}
	}
	return strings.Join(bodyLines, "\n")
}

// resolveImportPath converts a Python import path like "app.schemas.tags" to an absolute file path.
func resolveImportPath(importPath, projectRoot string) string {
	// "app.schemas.tags" -> "app/schemas/tags.py"
	relative := strings.ReplaceAll(importPath, ".", string(filepath.Separator)) + ".py"
	return filepath.Join(projectRoot, relative)
}

// resolvePydanticModel finds the schema class definition, first in the route file itself,
// then by following imports.
func resolvePydanticModel(modelName, routeText, routeFile string) map[string]interface{} {
	// Step 1: look in the same file
	body := extractClassBody(routeText, modelName)
	if body != "" {
		fields := parsePydanticClass(body)
		if len(fields) > 0 {
			return fields
		}
	}

	// Step 2: scan import lines like "from app.schemas.tags import TagCreate, TagResponse"
	projectRoot := findProjectRoot(routeFile)
	importRegex := regexp.MustCompile(`(?m)^from\s+([\w.]+)\s+import\s+(.+)$`)
	for _, m := range importRegex.FindAllStringSubmatch(routeText, -1) {
		if len(m) < 3 {
			continue
		}
		importedNames := m[2]
		// Check if modelName is among the imported names
		found := false
		for _, name := range strings.Split(importedNames, ",") {
			name = strings.TrimSpace(name)
			// Handle "from X import Y as Z"
			if strings.Contains(name, " as ") {
				parts := strings.SplitN(name, " as ", 2)
				name = strings.TrimSpace(parts[0])
			}
			if name == modelName {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		// Resolve the module path to a file
		modulePath := m[1]
		// Handle relative imports (e.g., "..schemas.tags") — convert to absolute
		if strings.HasPrefix(modulePath, ".") {
			// Fallback: search the project root for the class
			break
		}
		absFile := resolveImportPath(modulePath, projectRoot)
		importedContent, err := os.ReadFile(absFile)
		if err != nil {
			// Try alternate: search all .py files in project for the class
			break
		}
		body = extractClassBody(string(importedContent), modelName)
		if body != "" {
			fields := parsePydanticClass(body)
			if len(fields) > 0 {
				return fields
			}
		}
	}

	// Step 3: fallback — grep all .py files in the project for the class definition
	var result map[string]interface{}
	filepath.Walk(projectRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil || result != nil {
			return nil
		}
		if fi.IsDir() {
			name := fi.Name()
			if name == "__pycache__" || name == "venv" || name == ".venv" || name == "migrations" || name == "tests" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".py") || path == routeFile {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		body := extractClassBody(string(data), modelName)
		if body != "" {
			fields := parsePydanticClass(body)
			if len(fields) > 0 {
				result = fields
			}
		}
		return nil
	})

	return result
}

// tsFieldDefault uses the field name to guess a sensible JSON default value.
func tsFieldDefault(name string) interface{} {
	low := strings.ToLower(name)
	if strings.Contains(low, "email") || strings.Contains(low, "mail") {
		return ""
	}
	if strings.Contains(low, "count") || strings.Contains(low, "amount") ||
		strings.Contains(low, "price") || strings.Contains(low, "age") ||
		strings.Contains(low, "number") || strings.Contains(low, "qty") ||
		strings.Contains(low, "limit") || strings.Contains(low, "offset") {
		return 0
	}
	if strings.HasPrefix(low, "is_") || strings.HasPrefix(low, "has_") ||
		strings.HasPrefix(low, "can_") || strings.HasPrefix(low, "enable") ||
		strings.Contains(low, "active") || strings.Contains(low, "flag") ||
		strings.HasSuffix(low, "enabled") {
		return false
	}
	if strings.Contains(low, "tags") || strings.Contains(low, "items") ||
		strings.Contains(low, "list") || strings.Contains(low, "array") ||
		strings.HasSuffix(low, "ids") {
		return []interface{}{}
	}
	return ""
}

// detectSchema reads the route's source file and attempts to build a JSON body template.
func detectSchema(route scanner.Route, info detector.StackInfo) string {
	if route.FilePath == "" {
		return ""
	}

	content, err := os.ReadFile(route.FilePath)
	if err != nil {
		return ""
	}
	text := string(content)

	var schema map[string]interface{}

	// ──────────────────────────────────────────────────────────────
	// FastAPI / Python path
	// ──────────────────────────────────────────────────────────────
	if info.Framework == "FastAPI" || info.Language == "Python" {
		// Step 1: Find the route handler function for this specific HTTP method + path.
		// Pattern matches: @router.post("/tags") or @app.post("/tags")
		// followed by async def / def handler(...)
		urlPath := route.Path
		method := strings.ToLower(route.Method)

		// Regex: find the decorator line then capture the function signature on following lines
		decoratorRe := regexp.MustCompile(
			`(?i)@(?:router|app)\.` + regexp.QuoteMeta(method) +
				`\s*\(\s*["']` + regexp.QuoteMeta(urlPath) + `["']`)
		decLoc := decoratorRe.FindStringIndex(text)
		if decLoc == nil {
			// Try without quotes match (path might have placeholders)
			decoratorRe2 := regexp.MustCompile(`(?i)@(?:router|app)\.` + regexp.QuoteMeta(method) + `\s*\(`)
			decLoc = decoratorRe2.FindStringIndex(text)
		}

		if decLoc != nil {
			// Find the "def " or "async def " after the decorator block
			afterDecorator := text[decLoc[1]:]
			defRe := regexp.MustCompile(`(?s)(?:async\s+)?def\s+\w+\s*\(([^)]*(?:\([^)]*\)[^)]*)*)\)`)
			defMatch := defRe.FindStringSubmatch(afterDecorator)
			if len(defMatch) >= 2 {
				params := defMatch[1]
				// Parse comma-separated params, respecting nested parens
				for _, param := range splitParams(params) {
					param = strings.TrimSpace(param)
					if param == "" || param == "*" || param == "**kwargs" || param == "*args" {
						continue
					}
					// Get param name
					colonIdx := strings.Index(param, ":")
					if colonIdx < 0 {
						continue
					}
					paramName := strings.TrimSpace(param[:colonIdx])
					if skipParamNames[paramName] {
						continue
					}
					typeAnnotation := strings.TrimSpace(param[colonIdx+1:])
					// Strip default value
					if eqIdx := strings.Index(typeAnnotation, "="); eqIdx > 0 {
						typeAnnotation = strings.TrimSpace(typeAnnotation[:eqIdx])
					}
					// Skip if it's a known infrastructure type
					skip := false
					for _, kw := range skipParamTypeKeywords {
						if strings.HasPrefix(typeAnnotation, kw) {
							skip = true
							break
						}
					}
					if skip {
						continue
					}
					// This looks like a Pydantic body model!
					modelName := typeAnnotation
					fields := resolvePydanticModel(modelName, text, route.FilePath)
					if len(fields) > 0 {
						schema = fields
						break
					}
				}
			}
		}
	} else {
		// ──────────────────────────────────────────────────────────────
		// Next.js / TypeScript / Node.js path
		// ──────────────────────────────────────────────────────────────
		method := strings.ToUpper(route.Method)

		// Narrow to the handler function for this specific method
		handlerRe := regexp.MustCompile(
			`(?s)export\s+(?:async\s+)?function\s+` + method + `\s*\([^{]*\{(.+?)(?:^export\s+|\z)`)
		handlerMatch := handlerRe.FindStringSubmatch(text)
		handlerText := text // fallback to full file
		if len(handlerMatch) >= 2 {
			handlerText = handlerMatch[1]
		}

		// Pattern A: const { field1, field2 } = await req.json()  or  req.body
		patA := regexp.MustCompile(`const\s+\{\s*([^}]+)\s*\}\s*=\s*(?:await\s+(?:req|request)\.json\(\)|(?:req|request)\.body)`)
		if m := patA.FindStringSubmatch(handlerText); len(m) >= 2 {
			schema = make(map[string]interface{})
			for _, f := range strings.Split(m[1], ",") {
				key := strings.TrimSpace(f)
				// Handle renamed fields: "name: n" -> use "name"
				if idx := strings.Index(key, ":"); idx > 0 {
					key = strings.TrimSpace(key[:idx])
				}
				if key != "" {
					schema[key] = tsFieldDefault(key)
				}
			}
		}

		// Pattern B: const body = await req.json(); body.field1
		if schema == nil {
			patB := regexp.MustCompile(`const\s+(\w+)\s*=\s*await\s+(?:req|request)\.json\(\)`)
			if bm := patB.FindStringSubmatch(handlerText); len(bm) >= 2 {
				varName := bm[1]
				fieldRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\.([A-Za-z0-9_]+)`)
				matches := fieldRe.FindAllStringSubmatch(handlerText, -1)
				if len(matches) > 0 {
					schema = make(map[string]interface{})
					for _, fm := range matches {
						schema[fm[1]] = tsFieldDefault(fm[1])
					}
				}
			}
		}

		// Pattern C: Zod schema — z.object({ field: z.string(), ... })
		if schema == nil {
			zodRe := regexp.MustCompile(`z\.object\(\{([^}]+)\}`)
			if zm := zodRe.FindStringSubmatch(text); len(zm) >= 2 {
				schema = make(map[string]interface{})
				// Each entry: "  field: z.string()" or "  field: z.number()"
				entryRe := regexp.MustCompile(`(\w+)\s*:\s*z\.(\w+)\(`)
				for _, em := range entryRe.FindAllStringSubmatch(zm[1], -1) {
					if len(em) >= 3 {
						field := em[1]
						zodType := strings.ToLower(em[2])
						switch zodType {
						case "number":
							schema[field] = 0
						case "boolean":
							schema[field] = false
						case "array":
							schema[field] = []interface{}{}
						case "object":
							schema[field] = map[string]interface{}{}
						default:
							schema[field] = tsFieldDefault(field)
						}
					}
				}
			}
		}

		// Pattern D: Nuxt 3 / Nitro - readBody(event) or readValidatedBody
		if schema == nil {
			patNuxtA := regexp.MustCompile(`const\s+\{\s*([^}]+)\s*\}\s*=\s*await\s+(?:readBody|readValidatedBody)\b`)
			if nm := patNuxtA.FindStringSubmatch(handlerText); len(nm) >= 2 {
				schema = make(map[string]interface{})
				for _, f := range strings.Split(nm[1], ",") {
					key := strings.TrimSpace(f)
					if idx := strings.Index(key, ":"); idx > 0 {
						key = strings.TrimSpace(key[:idx])
					}
					if key != "" {
						schema[key] = tsFieldDefault(key)
					}
				}
			}

			if schema == nil {
				patNuxtB := regexp.MustCompile(`const\s+(\w+)\s*=\s*await\s+(?:readBody|readValidatedBody)\b`)
				if bm := patNuxtB.FindStringSubmatch(handlerText); len(bm) >= 2 {
					varName := bm[1]
					fieldRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\.([A-Za-z0-9_]+)`)
					matches := fieldRe.FindAllStringSubmatch(handlerText, -1)
					if len(matches) > 0 {
						schema = make(map[string]interface{})
						for _, fm := range matches {
							schema[fm[1]] = tsFieldDefault(fm[1])
						}
					}
				}
			}
		}

		// Pattern D: TypeScript interface — interface XBody { field: type; }
		if schema == nil {
			ifaceRe := regexp.MustCompile(`interface\s+\w+\s*\{([^}]+)\}`)
			if im := ifaceRe.FindStringSubmatch(text); len(im) >= 2 {
				schema = make(map[string]interface{})
				propRe := regexp.MustCompile(`(\w+)\??\s*:\s*(\w+)`)
				for _, pm := range propRe.FindAllStringSubmatch(im[1], -1) {
					if len(pm) >= 3 {
						field := pm[1]
						tsType := strings.ToLower(pm[2])
						switch tsType {
						case "number":
							schema[field] = 0
						case "boolean":
							schema[field] = false
						case "array":
							schema[field] = []interface{}{}
						default:
							schema[field] = tsFieldDefault(field)
						}
					}
				}
			}
		}

		// Pattern E: requestSchema.parse(body) — look for the schema variable name
		if schema == nil {
			parseRe := regexp.MustCompile(`(\w+Schema)\s*\.\s*parse\b`)
			if pm := parseRe.FindStringSubmatch(handlerText); len(pm) >= 2 {
				schemaVar := pm[1]
				// Find its Zod definition in the file
				varDefRe := regexp.MustCompile(`const\s+` + regexp.QuoteMeta(schemaVar) + `\s*=\s*z\.object\(\{([^}]+)\}`)
				if vdm := varDefRe.FindStringSubmatch(text); len(vdm) >= 2 {
					schema = make(map[string]interface{})
					entryRe := regexp.MustCompile(`(\w+)\s*:\s*z\.(\w+)\(`)
					for _, em := range entryRe.FindAllStringSubmatch(vdm[1], -1) {
						if len(em) >= 3 {
							field := em[1]
							zodType := strings.ToLower(em[2])
							switch zodType {
							case "number":
								schema[field] = 0
							case "boolean":
								schema[field] = false
							case "array":
								schema[field] = []interface{}{}
							case "object":
								schema[field] = map[string]interface{}{}
							default:
								schema[field] = tsFieldDefault(field)
							}
						}
					}
				}
			}
		}
		// Pattern C: NestJS @Body() DTO parsing
		if schema == nil {
			patC := regexp.MustCompile(`@Body\(\)\s+\w+\s*:\s*([A-Za-z0-9_]+)`)
			if cm := patC.FindStringSubmatch(handlerText); len(cm) >= 2 {
				dtoName := cm[1]
				fields := resolveTSModel(dtoName, text, route.FilePath)
				if len(fields) > 0 {
					schema = fields
				}
			}
		}
	}

	if schema != nil && len(schema) > 0 {
		b, err := json.MarshalIndent(schema, "", "  ")
		if err == nil {
			return string(b)
		}
	}

	// Fallback: empty — never show {"key": "value"}
	return ""
}

// splitParams splits a Go-style or Python-style function parameter list
// by commas while respecting nested parentheses.
func splitParams(params string) []string {
	var result []string
	depth := 0
	start := 0
	for i, ch := range params {
		switch ch {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				result = append(result, params[start:i])
				start = i + 1
			}
		}
	}
	if start < len(params) {
		result = append(result, params[start:])
	}
	return result
}

func initialRequestEditorModel(route scanner.Route, info detector.StackInfo, template string) requestEditorModel {
	ti := textinput.New()
	ti.Placeholder = "Bearer ey..."
	ti.CharLimit = 4096
	ti.Width = 54

	ta := textarea.New()
	ta.Placeholder = "{\n  \"key\": \"value\"\n}"
	ta.SetHeight(10)
	ta.SetWidth(60)
	ta.ShowLineNumbers = false

	hasBody := route.Method == "POST" || route.Method == "PUT" || route.Method == "PATCH"
	if !hasBody {
		ta.Placeholder = "No body for this method"
	} else if template != "" {
		ta.SetValue(template)
	}

	var token string
	if envData, err := os.ReadFile(".env"); err == nil {
		lines := strings.Split(string(envData), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.ToUpper(parts[0])
				if strings.Contains(key, "TOKEN") || strings.Contains(key, "KEY") ||
					strings.Contains(key, "SECRET") || strings.Contains(key, "AUTH") {
					val := strings.Trim(parts[1], `"' `)
					if !strings.HasPrefix(strings.ToLower(val), "bearer") {
						token = "Bearer " + val
					} else {
						token = val
					}
					break
				}
			}
		}
	}
	if token != "" {
		ti.SetValue(token)
	}

	return requestEditorModel{
		route:      route,
		info:       info,
		authInput:  ti,
		bodyInput:  ta,
		focusIndex: 0,
		hasBody:    hasBody,
		statusMessage: "",
	}
}

func (m requestEditorModel) Init() tea.Cmd {
	return nil
}

func (m requestEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case clearStatusMsg:
		m.statusMessage = ""
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.focusIndex == 1 && m.hasBody {
				// Copy Body
				err := Copy(m.bodyInput.Value())
				if err != nil {
					m.statusMessage = "✗ Clipboard Unavailable"
				} else {
					m.statusMessage = "✓ Copied"
				}
				return m, tickClearStatus()
			}
			fallthrough
		case tea.KeyEsc:
			if m.isEditingAuth {
				m.isEditingAuth = false
				m.authInput.Blur()
			} else {
				m.canceled = true
				return m, tea.Quit
			}
		case tea.KeyCtrlS, tea.KeyF5:
			m.submitted = true
			return m, tea.Quit
		case tea.KeyTab, tea.KeyShiftTab:
			if m.hasBody {
				if m.focusIndex == 0 {
					m.focusIndex = 1
					m.isEditingAuth = false
					m.authInput.Blur()
					m.bodyInput.Focus()
				} else {
					m.focusIndex = 0
					m.bodyInput.Blur()
				}
			}
		case tea.KeyEnter:
			if m.focusIndex == 0 && !m.isEditingAuth {
				m.isEditingAuth = true
				m.authInput.Focus()
				return m, textinput.Blink
			}
		case tea.KeyCtrlV:
			if m.focusIndex == 1 && m.hasBody {
				text, err := Paste()
				if err != nil || text == "" {
					if err != nil {
						m.statusMessage = "✗ Clipboard Unavailable"
					} else {
						m.statusMessage = "⚠ Clipboard Empty"
					}
				} else {
					// We need to insert text into the textarea at the cursor position
					m.bodyInput.InsertString(text)
					m.statusMessage = "✓ Pasted"
				}
				return m, tickClearStatus()
			}
		}
	}

	if m.isEditingAuth {
		m.authInput, cmd = m.authInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.focusIndex == 1 && m.hasBody {
		m.bodyInput, cmd = m.bodyInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m requestEditorModel) View() string {
	var s strings.Builder
	s.WriteString("\n")

	// Header
	header := lipgloss.JoinHorizontal(lipgloss.Top, MethodBadge(m.route.Method), "  ", TextValue.Render(m.route.Path))
	s.WriteString(BaseLayout.Render(header))
	s.WriteString("\n\n")

	var cardRows []string

	// Auth Section
	authLabel := "Authorization"
	if m.focusIndex == 0 {
		authLabel = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render("› " + authLabel)
	} else {
		authLabel = "  " + authLabel
	}

	if m.isEditingAuth {
		cardRows = append(cardRows, AlignedRow(authLabel, m.authInput.View()))
	} else {
		authVal := m.authInput.Value()
		masked := "None"
		if authVal != "" {
			if len(authVal) > 6 {
				masked = authVal[:6] + "••••••••"
			} else {
				masked = "••••••••"
			}
		}

		authStr := fmt.Sprintf("%s [change]", masked)
		if m.focusIndex == 0 {
			authStr = lipgloss.NewStyle().Foreground(ColorCyan).Render(authStr)
		} else {
			authStr = MutedText(authStr)
		}
		cardRows = append(cardRows, AlignedRow(authLabel, authStr))
	}

	cardRows = append(cardRows, AlignedRow("  Content-Type", TextValue.Render("application/json")))
	
	cardRows = append(cardRows, Divider())

	// Body Section
	bodyLabel := "Body (JSON)"
	if m.focusIndex == 1 {
		bodyLabel = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render("› " + bodyLabel)
	} else {
		bodyLabel = "  " + bodyLabel
	}
	cardRows = append(cardRows, bodyLabel)
	cardRows = append(cardRows, "")
	
	bodyView := lipgloss.NewStyle().PaddingLeft(4).Render(m.bodyInput.View())
	cardRows = append(cardRows, bodyView)
	
	cardContent := strings.Join(cardRows, "\n")
	s.WriteString(BaseLayout.Render(CardStyle.Render(cardContent)))
	s.WriteString("\n")
	footer := KeyHintBar([]Key{
		{Name: "Tab", Desc: "Switch"},
		{Name: "Ctrl+C/V", Desc: "Copy/Paste"},
		{Name: "Ctrl+S", Desc: "Send"},
		{Name: "Esc", Desc: "Cancel"},
	})

	if m.statusMessage != "" {
		statusStr := lipgloss.NewStyle().Foreground(ColorYellow).Render(m.statusMessage)
		if strings.HasPrefix(m.statusMessage, "✓") {
			statusStr = lipgloss.NewStyle().Foreground(ColorGreen).Render(m.statusMessage)
		} else if strings.HasPrefix(m.statusMessage, "✗") {
			statusStr = lipgloss.NewStyle().Foreground(ColorRed).Render(m.statusMessage)
		}
		footer = lipgloss.JoinHorizontal(lipgloss.Center, footer, "  │  ", statusStr)
	}

	s.WriteString(footer)
	s.WriteString("\n")

	return s.String()
}

func BuildRequest(route scanner.Route, info detector.StackInfo) executor.Request {
	paramRegex := regexp.MustCompile(`\{([^}]+)\}`)
	matches := paramRegex.FindAllStringSubmatch(route.Path, -1)

	if len(matches) > 0 {
		fmt.Println()
		reader := bufio.NewReader(os.Stdin)
		for _, match := range matches {
			paramName := match[1]
			fmt.Printf("  Enter value for {%s}: ", paramName)
			val, _ := reader.ReadString('\n')
			val = strings.TrimSpace(val)
			route.Path = strings.Replace(route.Path, match[0], val, 1)
		}
	}

	baseURL := "http://localhost:" + info.Port
	if customURL := os.Getenv("TESTIFY_URL"); customURL != "" {
		baseURL = customURL
	} else if customPort := os.Getenv("PORT"); customPort != "" {
		baseURL = "http://localhost:" + customPort
	}

	req := executor.Request{
		Method:  route.Method,
		Path:    route.Path,
		BaseURL: baseURL,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	template := ""
	if req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" {
		template = detectSchema(route, info)
	}

	if template != "" {
		req.Body = template
	}

	return req
}

func tickClearStatus() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}
