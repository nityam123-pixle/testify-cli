package detector

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type StackInfo struct {
	Framework string
	Language  string
	Port      string
	HasDotEnv bool
	HasConvex     bool
	HasBetterAuth bool
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func Detect(dir string) StackInfo {
	info := StackInfo{}
	var defaultPort string

	// Check Node.js
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err == nil {
		var pkg packageJSON
		json.Unmarshal(data, &pkg)
		all := merge(pkg.Dependencies, pkg.DevDependencies)

		if _, ok := all["express"]; ok {
			info.Framework, info.Language = "Express", "Node.js"
			defaultPort = "3000"
		} else if _, ok := all["fastify"]; ok {
			info.Framework, info.Language = "Fastify", "Node.js"
			defaultPort = "3000"
		} else if _, ok := all["@nestjs/core"]; ok {
			info.Framework, info.Language = "NestJS", "Node.js"
			defaultPort = "3000"
		} else if _, ok := all["hono"]; ok {
			info.Framework, info.Language = "Hono", "Node.js"
			defaultPort = "3000"
		} else if _, ok := all["next"]; ok {
			info.Framework, info.Language = "Next.js", "Node.js"
			defaultPort = "3000"
		}

		if _, ok := all["convex"]; ok {
			info.HasConvex = true
		}

		if _, ok := all["better-auth"]; ok {
			info.HasBetterAuth = true
		}
	}

	// Check Python
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		content, _ := os.ReadFile(filepath.Join(dir, "requirements.txt"))
		s := string(content)
		if strings.Contains(s, "fastapi") {
			info.Framework, info.Language = "FastAPI", "Python"
			defaultPort = "8000"
		} else if strings.Contains(s, "flask") {
			info.Framework, info.Language = "Flask", "Python"
			defaultPort = "5000"
		} else if strings.Contains(s, "django") {
			info.Framework, info.Language = "Django", "Python"
			defaultPort = "8000"
		}
	}

	// Check Go
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		content, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
		s := string(content)
		if strings.Contains(s, "gin-gonic") {
			info.Framework, info.Language = "Gin", "Go"
			defaultPort = "8080"
		} else if strings.Contains(s, "labstack/echo") {
			info.Framework, info.Language = "Echo", "Go"
			defaultPort = "8080"
		}
	}

	// Check environment files for ports
	envFiles := []string{".env", ".env.local", ".env.development"}
	for _, f := range envFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			info.HasDotEnv = true
		}
	}

	envPort := readPortFromEnvs(dir, envFiles...)
	if envPort != "" {
		info.Port = envPort
	}

	// FastAPI specific uvicorn check
	if info.Port == "" && info.Framework == "FastAPI" {
		uvPort := findUvicornPort(dir)
		if uvPort != "" {
			info.Port = uvPort
		}
	}

	// Fallback to default
	if info.Port == "" {
		info.Port = defaultPort
	}

	return info
}

func merge(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

func readPortFromEnvs(dir string, files ...string) string {
	keys := []string{"PORT=", "APP_PORT=", "SERVER_PORT=", "BACKEND_PORT="}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			for _, key := range keys {
				if strings.HasPrefix(line, key) {
					port := strings.TrimPrefix(line, key)
					port = strings.Trim(port, `"' `)
					if idx := strings.Index(port, "#"); idx != -1 {
						port = strings.TrimSpace(port[:idx])
					}
					if port != "" {
						return port
					}
				}
			}
		}
	}
	return ""
}

func findUvicornPort(dir string) string {
	var port string
	var errStop = errors.New("stop")
	
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "venv" || name == ".venv" || name == "node_modules" || name == "__pycache__" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		
		if strings.HasSuffix(path, ".py") {
			data, err := os.ReadFile(path)
			if err == nil {
				content := string(data)
				if strings.Contains(content, "uvicorn.run(") {
					re := regexp.MustCompile(`port\s*=\s*(\d+)`)
					matches := re.FindStringSubmatch(content)
					if len(matches) > 1 {
						port = matches[1]
						return errStop
					}
				}
			}
		}
		return nil
	})

	if err == errStop {
		return port
	}
	return port
}

func hasProjectFile(dir string) bool {
	files := []string{"package.json", "requirements.txt", "go.mod", "pyproject.toml"}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}

func FindProjectRoots(dir string) []string {
	if hasProjectFile(dir) {
		return []string{dir}
	}

	var roots []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{dir}
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && entry.Name() != "node_modules" {
			subDir := filepath.Join(dir, entry.Name())
			if hasProjectFile(subDir) {
				roots = append(roots, subDir)
			}
		}
	}

	if len(roots) == 0 {
		return []string{dir}
	}

	sort.Slice(roots, func(i, j int) bool {
		nameI := strings.ToLower(filepath.Base(roots[i]))
		nameJ := strings.ToLower(filepath.Base(roots[j]))

		scoreI := scoreRootName(nameI)
		scoreJ := scoreRootName(nameJ)

		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		return nameI < nameJ
	})

	return roots
}

func scoreRootName(name string) int {
	if strings.Contains(name, "api") || strings.Contains(name, "back") || strings.Contains(name, "server") {
		return 2
	}
	if strings.Contains(name, "front") || strings.Contains(name, "web") || strings.Contains(name, "ui") || strings.Contains(name, "client") {
		return 1
	}
	return 0
}
