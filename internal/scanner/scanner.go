package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nityam123-pixle/testify-cli/internal/detector"
)

type Route struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	File     string `json:"file,omitempty"`
	FilePath string `json:"filePath,omitempty"`
}

type TestifyConfig struct {
	CustomRoutes []Route `json:"customRoutes"`
}

func loadCustomRoutes(dir string) []Route {
	configPath := filepath.Join(dir, "testify.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var config TestifyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil
	}

	for i := range config.CustomRoutes {
		if config.CustomRoutes[i].File == "" {
			config.CustomRoutes[i].File = "testify.json (custom)"
			config.CustomRoutes[i].FilePath = "testify.json"
		}
	}

	return config.CustomRoutes
}

var expressPattern = regexp.MustCompile(
	`(?i)(router|app|route)\.(get|post|put|delete|patch)\s*\(\s*['"` + "`" + `]([^'"` + "`" + `]+)`)

var fastAPIPattern = regexp.MustCompile(
	`@(app|router)\.(get|post|put|delete|patch)\s*\(\s*["']([^"']+)["']`)

var nextAppPattern = regexp.MustCompile(`\bexport\s+(?:async\s+)?(?:function|const|let|var)\s+(GET|POST|PUT|DELETE|PATCH)\b`)
var nextExportListPattern = regexp.MustCompile(`\bexport\s+(?:const\s+|let\s+|var\s+)?\{([^}]+)\}`)

var nestjsControllerPattern = regexp.MustCompile(`@Controller\(\s*(?:['"]([^'"]*)['"])?\s*\)`)
var nestjsMethodPattern = regexp.MustCompile(`@(Get|Post|Put|Delete|Patch)\(\s*(?:['"]([^'"]*)['"])?\s*\)`)

func ScanRoutes(dir string, stackInfo detector.StackInfo) []Route {
	var routes []Route

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if name == ".git" {
				return filepath.SkipDir
			}

			switch stackInfo.Framework {
			case "Next.js":
				if name == ".next" || name == "node_modules" || name == "public" {
					return filepath.SkipDir
				}
			case "FastAPI", "Flask", "Django":
				if name == "__pycache__" || name == "venv" || name == ".venv" || name == "env" || name == "migrations" || name == "tests" || name == "alembic" {
					return filepath.SkipDir
				}
			default: // Express, Fastify, Hono, Gin, NestJS etc.
				if name == "node_modules" || name == "dist" || name == "build" || name == ".next" || name == "coverage" || name == "__tests__" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if stackInfo.Framework == "Next.js" {
			name := info.Name()
			isAppRoute := name == "route.ts" || name == "route.js"
			isPagesRoute := (strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".js")) && strings.Contains(path, "/api/") && !isAppRoute

			if isAppRoute || isPagesRoute {
				content, err := os.ReadFile(path)
				if err == nil {
					text := string(content)
					var apiPath string
					if isAppRoute {
						apiPath = extractNextAppPath(path)
					} else {
						apiPath = extractNextPagesPath(path)
					}

					if isAppRoute {
						foundMethods := make(map[string]bool)
						for _, m := range nextAppPattern.FindAllStringSubmatch(text, -1) {
							if len(m) > 1 {
								foundMethods[m[1]] = true
							}
						}
						for _, m := range nextExportListPattern.FindAllStringSubmatch(text, -1) {
							if len(m) > 1 {
								for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
									if strings.Contains(m[1], method) {
										foundMethods[method] = true
									}
								}
							}
						}

						for method := range foundMethods {
							routes = append(routes, Route{
								Method:   strings.ToUpper(method),
								Path:     apiPath,
								File:     shortenPath(path),
								FilePath: path,
							})
						}
					} else if isPagesRoute {
						methodsFound := false
						for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
							if strings.Contains(text, "req.method === '"+method+"'") || strings.Contains(text, `req.method === "`+method+`"`) {
								routes = append(routes, Route{
									Method:   strings.ToUpper(method),
									Path:     apiPath,
									File:     shortenPath(path),
									FilePath: path,
								})
								methodsFound = true
							}
						}
						if !methodsFound {
							routes = append(routes, Route{
								Method:   "GET",
								Path:     apiPath,
								File:     shortenPath(path),
								FilePath: path,
							})
						}
					}
				}
			}
			return nil
		}

		for _, e := range getExtensions(stackInfo.Framework) {
			if filepath.Ext(path) == e {
				routes = append(routes, scanFile(path, stackInfo.Framework)...)
				break
			}
		}
		return nil
	})

	if stackInfo.HasBetterAuth {
		routes = append(routes, 
			Route{Method: "POST", Path: "/api/auth/sign-in/email", File: "better-auth (auto-detected)", FilePath: "better-auth"},
			Route{Method: "POST", Path: "/api/auth/sign-up/email", File: "better-auth (auto-detected)", FilePath: "better-auth"},
			Route{Method: "POST", Path: "/api/auth/sign-out", File: "better-auth (auto-detected)", FilePath: "better-auth"},
			Route{Method: "GET", Path: "/api/auth/session", File: "better-auth (auto-detected)", FilePath: "better-auth"},
		)
	}

	customRoutes := loadCustomRoutes(dir)
	routes = append(routes, customRoutes...)

	return routes
}

func scanFile(pathParam, framework string) []Route {
	content, err := os.ReadFile(pathParam)
	if err != nil {
		return nil
	}

	if framework == "NestJS" {
		text := string(content)
		var prefix string
		ctrlMatch := nestjsControllerPattern.FindStringSubmatch(text)
		if len(ctrlMatch) > 1 {
			prefix = ctrlMatch[1]
		}
		if prefix != "" && prefix[0] != '/' {
			prefix = "/" + prefix
		}

		var routes []Route
		for _, m := range nestjsMethodPattern.FindAllStringSubmatch(text, -1) {
			method := m[1]
			path := ""
			if len(m) > 2 {
				path = m[2]
			}
			
			if path != "" && path[0] != '/' {
				path = "/" + path
			}
			fullPath := prefix + path
			if fullPath == "" {
				fullPath = "/"
			}

			routes = append(routes, Route{
				Method:   strings.ToUpper(method),
				Path:     fullPath,
				File:     shortenPath(pathParam),
				FilePath: pathParam,
			})
		}
		return routes
	}

	pattern := expressPattern
	if framework == "FastAPI" || framework == "Flask" {
		pattern = fastAPIPattern
	}

	var routes []Route
	for _, m := range pattern.FindAllStringSubmatch(string(content), -1) {
		if len(m) >= 4 {
			routes = append(routes, Route{
				Method:   strings.ToUpper(m[2]),
				Path:     m[3],
				File:     shortenPath(pathParam),
				FilePath: pathParam,
			})
		}
	}
	return routes
}

func getExtensions(framework string) []string {
	switch framework {
	case "FastAPI", "Flask", "Django":
		return []string{".py"}
	case "Gin", "Echo":
		return []string{".go"}
	default:
		return []string{".js", ".ts", ".mjs"}
	}
}

func shortenPath(path string) string {
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) > 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}
	return path
}

func extractNextAppPath(path string) string {
	path = filepath.ToSlash(path)
	idx := strings.LastIndex(path, "/app/")
	if idx != -1 {
		rel := path[idx+4:] // keep the leading '/'
		rel = strings.TrimSuffix(rel, "/route.ts")
		rel = strings.TrimSuffix(rel, "/route.js")
		if rel == "" {
			return "/"
		}
		return rel
	}
	return "/"
}

func extractNextPagesPath(path string) string {
	path = filepath.ToSlash(path)
	idx := strings.LastIndex(path, "/api/")
	if idx != -1 {
		rel := path[idx:] // keep "/api/"
		rel = strings.TrimSuffix(rel, ".ts")
		rel = strings.TrimSuffix(rel, ".js")
		if strings.HasSuffix(rel, "/index") {
			rel = strings.TrimSuffix(rel, "/index")
		}
		if rel == "" {
			return "/"
		}
		return rel
	}
	return "/"
}
