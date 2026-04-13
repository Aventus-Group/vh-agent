package analyzesvc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ---------- Configuration ----------

var (
	maxTreeDepth     = 6
	maxTotalFiles    = 500
	maxReadSize      = int64(100_000)
	maxSourceAnalyze = 50

	configPatterns = []string{
		"package.json", "tsconfig.json", "tsconfig.*.json",
		"vite.config.*", "next.config.*", "nuxt.config.*", "svelte.config.*",
		"webpack.config.*", "rollup.config.*", "esbuild.config.*",
		"tailwind.config.*", "postcss.config.*",
		".env.example", ".env.sample",
		"nginx.conf", "nginx/*.conf",
		"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml",
		"Dockerfile", "Dockerfile.*",
		"requirements.txt", "Pipfile", "pyproject.toml", "setup.py", "setup.cfg",
		"composer.json", "Gemfile",
		".eslintrc*", ".prettierrc*",
		"Makefile", "Taskfile.yml",
		"vercel.json", "netlify.toml", "fly.toml",
		"prisma/schema.prisma",
		"drizzle.config.*",
	}

	sourceExtensions = map[string]bool{
		".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".py": true, ".rb": true, ".go": true, ".rs": true,
		".php": true, ".java": true, ".cs": true,
		".vue": true, ".svelte": true, ".astro": true,
	}

	skipDirs = map[string]bool{
		"node_modules": true, ".git": true, ".next": true, ".nuxt": true,
		"dist": true, "build": true, ".cache": true, "__pycache__": true,
		".venv": true, "venv": true, "vendor": true, ".svelte-kit": true,
		"coverage": true, ".turbo": true, ".output": true, ".vercel": true,
		".netlify": true, "target": true, "bin": true, "obj": true,
		".claude": true, ".idea": true, ".vscode": true,
	}

	entryPointPatterns = []string{
		"src/main.ts", "src/main.tsx", "src/main.js", "src/main.jsx",
		"src/index.ts", "src/index.tsx", "src/index.js", "src/index.jsx",
		"src/App.tsx", "src/App.jsx", "src/App.vue", "src/App.svelte",
		"src/app/page.tsx", "src/app/layout.tsx",
		"src/app/main.tsx", "src/app/main.ts",
		"pages/index.tsx", "pages/index.jsx", "pages/_app.tsx",
		"app.py", "main.py", "manage.py", "wsgi.py", "asgi.py",
		"index.php", "public/index.php",
		"main.go", "cmd/main.go", "cmd/server/main.go",
		"server.ts", "server.js", "app.ts", "app.js",
		"index.ts", "index.js",
		"index.html",
		"next.config.js", "next.config.mjs", "next.config.ts",
		"vite.config.ts", "vite.config.js",
		"nuxt.config.ts", "astro.config.mjs",
	}

	routeDirPatterns = []string{
		"src/app", "src/pages", "src/routes", "src/views",
		"pages", "routes", "app/routes", "app/views",
	}

	// Regex for code analysis
	reImportTS  = regexp.MustCompile(`(?m)^import\s+.*?from\s+['"]([^'"]+)['"]`)
	reImportPy  = regexp.MustCompile(`(?m)^(?:from\s+(\S+)\s+import|import\s+(\S+))`)
	reExportTS  = regexp.MustCompile(`(?m)^export\s+(?:default\s+)?(?:function|class|const|let|var|interface|type|enum)\s+(\w+)`)
	reExportDef = regexp.MustCompile(`(?m)^export\s+default\s+(\w+)`)
	reFuncTS    = regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	reClassTS   = regexp.MustCompile(`(?m)^(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`)
	reConstTS   = regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)\s*[=:]`)
	reInterface = regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)`)
	reTypeAlias = regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)\s*[=<]`)
	reComponent = regexp.MustCompile(`(?m)^(?:export\s+)?(?:default\s+)?function\s+([A-Z]\w+)\s*\(`)
	reFuncPy    = regexp.MustCompile(`(?m)^(?:async\s+)?def\s+(\w+)`)
	reClassPy   = regexp.MustCompile(`(?m)^class\s+(\w+)`)
)

// ---------- Project Root Detection ----------

func findProjectRoot(basePath string) string {
	markers := []string{"package.json", "requirements.txt", "composer.json", "go.mod", "Cargo.toml", "Gemfile", "pom.xml"}

	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(basePath, m)); err == nil {
			return basePath
		}
	}

	entries, err := os.ReadDir(basePath)
	if err != nil {
		return basePath
	}

	for _, e := range entries {
		if !e.IsDir() || skipDirs[e.Name()] {
			continue
		}
		subDir := filepath.Join(basePath, e.Name())
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(subDir, m)); err == nil {
				return subDir
			}
		}
	}

	return basePath
}

// ---------- File Tree Walker ----------

func walkProject(root string, analysis *ProjectAnalysis) {
	analysis.FileTree = make([]FileEntry, 0, 256)

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}

		depth := strings.Count(rel, string(filepath.Separator))
		if depth > maxTreeDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			analysis.Stats.TotalDirs++
			analysis.FileTree = append(analysis.FileTree, FileEntry{
				Path: rel,
				Type: "dir",
			})
			return nil
		}

		analysis.Stats.TotalFiles++
		analysis.Stats.TotalSizeBytes += info.Size()

		if len(analysis.FileTree) < maxTotalFiles {
			entry := FileEntry{
				Path: rel,
				Type: "file",
				Size: info.Size(),
			}

			ext := strings.ToLower(filepath.Ext(rel))
			if sourceExtensions[ext] {
				analysis.Stats.SourceFiles++
				entry.Lines = countLines(path)
				analysis.Stats.TotalLines += entry.Lines
			}

			if isTestFile(rel) {
				analysis.Stats.TestFiles++
			}
			if isConfigFile(info.Name()) {
				analysis.Stats.ConfigFiles++
			}

			analysis.FileTree = append(analysis.FileTree, entry)
		}

		name := strings.ToLower(info.Name())
		switch {
		case name == "dockerfile" || strings.HasPrefix(name, "dockerfile."):
			analysis.Stats.HasDockerfile = true
		case strings.HasPrefix(name, "docker-compose") || name == "compose.yml" || name == "compose.yaml":
			analysis.Stats.HasDockerCompose = true
		case name == "nginx.conf" || strings.HasSuffix(name, ".nginx.conf"):
			analysis.Stats.HasNginxConfig = true
		case name == ".env" || name == ".env.local" || name == ".env.production":
			analysis.Stats.HasEnvFile = true
		}

		if depth > analysis.Stats.MaxDepth {
			analysis.Stats.MaxDepth = depth
		}

		return nil
	})
}

// ---------- Package JSON ----------

func readPackageJSON(root string) *PackageJSON {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return &pkg
}

// ---------- Stack Detection ----------

func detectStack(root string, pkg *PackageJSON) StackInfo {
	info := StackInfo{
		Framework:      "unknown",
		Language:        "unknown",
		PackageManager: "npm",
		Runtime:        "unknown",
	}

	if _, err := os.Stat(filepath.Join(root, "pnpm-lock.yaml")); err == nil {
		info.PackageManager = "pnpm"
	} else if _, err := os.Stat(filepath.Join(root, "yarn.lock")); err == nil {
		info.PackageManager = "yarn"
	} else if _, err := os.Stat(filepath.Join(root, "bun.lockb")); err == nil {
		info.PackageManager = "bun"
	} else if _, err := os.Stat(filepath.Join(root, "package-lock.json")); err == nil {
		info.PackageManager = "npm"
	}

	if pkg != nil {
		info.Runtime = "node"
		info.Language = detectLanguageFromPkg(root, pkg)
		deps := mergeMaps(pkg.Dependencies, pkg.DevDependencies)

		switch {
		case deps["next"] != "":
			info.Framework = "nextjs"
		case deps["nuxt"] != "" || deps["nuxt3"] != "":
			info.Framework = "nuxt"
		case deps["@sveltejs/kit"] != "":
			info.Framework = "sveltekit"
		case deps["astro"] != "":
			info.Framework = "astro"
		case deps["gatsby"] != "":
			info.Framework = "gatsby"
		case deps["remix"] != "" || deps["@remix-run/react"] != "":
			info.Framework = "remix"
		case deps["vite"] != "":
			info.Framework = "vite"
		case deps["react-scripts"] != "":
			info.Framework = "cra"
		case deps["express"] != "":
			info.Framework = "express"
		case deps["fastify"] != "":
			info.Framework = "fastify"
		case deps["@nestjs/core"] != "":
			info.Framework = "nestjs"
		case deps["koa"] != "":
			info.Framework = "koa"
		case deps["hono"] != "":
			info.Framework = "hono"
		case deps["react"] != "":
			info.Framework = "react"
		case deps["vue"] != "":
			info.Framework = "vue"
		case deps["svelte"] != "":
			info.Framework = "svelte"
		}
		return info
	}

	if _, err := os.Stat(filepath.Join(root, "requirements.txt")); err == nil {
		info.Runtime = "python"
		info.Language = "python"
		info.PackageManager = "pip"
		info.Framework = detectPythonFramework(root)
		return info
	}
	if _, err := os.Stat(filepath.Join(root, "pyproject.toml")); err == nil {
		info.Runtime = "python"
		info.Language = "python"
		info.PackageManager = "poetry"
		info.Framework = detectPythonFramework(root)
		return info
	}

	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		info.Runtime = "go"
		info.Language = "go"
		info.PackageManager = "go modules"
		info.Framework = "go"
		return info
	}

	if _, err := os.Stat(filepath.Join(root, "composer.json")); err == nil {
		info.Runtime = "php"
		info.Language = "php"
		info.PackageManager = "composer"
		info.Framework = detectPHPFramework(root)
		return info
	}

	if _, err := os.Stat(filepath.Join(root, "Gemfile")); err == nil {
		info.Runtime = "ruby"
		info.Language = "ruby"
		info.PackageManager = "bundler"
		info.Framework = "ruby"
		return info
	}

	if _, err := os.Stat(filepath.Join(root, "index.html")); err == nil {
		info.Runtime = "static"
		info.Language = "html"
		info.Framework = "static"
		return info
	}

	return info
}

func detectLanguageFromPkg(root string, pkg *PackageJSON) string {
	deps := mergeMaps(pkg.Dependencies, pkg.DevDependencies)
	if deps["typescript"] != "" {
		return "typescript"
	}
	if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
		return "typescript"
	}
	return "javascript"
}

func detectPythonFramework(root string) string {
	reqData, _ := os.ReadFile(filepath.Join(root, "requirements.txt"))
	req := strings.ToLower(string(reqData))

	switch {
	case strings.Contains(req, "django"):
		return "django"
	case strings.Contains(req, "fastapi"):
		return "fastapi"
	case strings.Contains(req, "flask"):
		return "flask"
	case strings.Contains(req, "streamlit"):
		return "streamlit"
	}
	return "python"
}

func detectPHPFramework(root string) string {
	data, _ := os.ReadFile(filepath.Join(root, "composer.json"))
	content := strings.ToLower(string(data))

	switch {
	case strings.Contains(content, "laravel"):
		return "laravel"
	case strings.Contains(content, "symfony"):
		return "symfony"
	case strings.Contains(content, "wordpress"):
		return "wordpress"
	}
	return "php"
}

// ---------- Config Files ----------

func readConfigs(root string, tree []FileEntry) []ConfigFile {
	configs := make([]ConfigFile, 0, 16)

	for _, entry := range tree {
		if entry.Type != "file" || !isConfigFile(filepath.Base(entry.Path)) || entry.Size > maxReadSize {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, entry.Path))
		if err != nil {
			continue
		}
		configs = append(configs, ConfigFile{Path: entry.Path, Content: string(content)})
	}

	// Also check nginx configs outside project root
	for _, p := range []string{
		"/etc/nginx/sites-enabled/default",
		"/etc/nginx/sites-available/default",
		"/etc/nginx/conf.d/app.conf",
		"/etc/nginx/nginx.conf",
	} {
		if content, err := os.ReadFile(p); err == nil {
			configs = append(configs, ConfigFile{Path: p, Content: string(content)})
		}
	}

	return configs
}

func isConfigFile(name string) bool {
	lower := strings.ToLower(name)
	for _, pattern := range configPatterns {
		if matched, _ := filepath.Match(strings.ToLower(pattern), lower); matched {
			return true
		}
	}
	return false
}

// ---------- Entry Points ----------

func detectEntryPoints(root string) []string {
	var found []string
	for _, ep := range entryPointPatterns {
		if _, err := os.Stat(filepath.Join(root, ep)); err == nil {
			found = append(found, ep)
		}
	}
	return found
}

// ---------- Routes ----------

func detectRoutes(root string) []string {
	var routes []string

	for _, dir := range routeDirPatterns {
		routeDir := filepath.Join(root, dir)
		if _, err := os.Stat(routeDir); err != nil {
			continue
		}

		_ = filepath.Walk(routeDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			ext := filepath.Ext(rel)
			if !sourceExtensions[ext] {
				return nil
			}
			name := filepath.Base(rel)
			if strings.HasPrefix(name, "_") || name == "layout.tsx" || name == "layout.ts" ||
				name == "loading.tsx" || name == "error.tsx" || name == "not-found.tsx" {
				return nil
			}
			routes = append(routes, "/"+strings.TrimPrefix(rel, dir+"/"))
			return nil
		})
	}

	sort.Strings(routes)
	if len(routes) > 50 {
		routes = routes[:50]
	}
	return routes
}

// ---------- Source Analysis ----------

func analyzeSourceFiles(root string, tree []FileEntry) []SourceFile {
	var sources []SourceFile

	analyzed := 0
	for _, entry := range tree {
		if analyzed >= maxSourceAnalyze {
			break
		}
		if entry.Type != "file" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Path))
		if !sourceExtensions[ext] || isTestFile(entry.Path) {
			continue
		}
		if entry.Size > maxReadSize || entry.Size == 0 {
			continue
		}

		content, err := os.ReadFile(filepath.Join(root, entry.Path))
		if err != nil {
			continue
		}

		src := analyzeFile(entry.Path, string(content), ext)
		if len(src.Declarations) > 0 || len(src.Exports) > 0 {
			sources = append(sources, src)
			analyzed++
		}
	}

	return sources
}

func analyzeFile(path, content, ext string) SourceFile {
	lines := strings.Count(content, "\n") + 1
	src := SourceFile{Path: path, Lines: lines}

	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".vue", ".svelte", ".astro":
		src.Imports = extractMatches(reImportTS, content, 1, 30)
		src.Exports = extractMatches(reExportTS, content, 1, 20)
		if defs := extractMatches(reExportDef, content, 1, 5); len(defs) > 0 {
			src.Exports = append(src.Exports, defs...)
		}
		src.Declarations = extractDeclarations(content)
	case ".py":
		imports1 := extractMatches(reImportPy, content, 1, 30)
		imports2 := extractMatches(reImportPy, content, 2, 30)
		src.Imports = append(imports1, imports2...)
		src.Declarations = extractPythonDeclarations(content)
	}

	return src
}

func extractDeclarations(content string) []Declaration {
	var decls []Declaration
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1
		if m := reComponent.FindStringSubmatch(line); len(m) > 1 {
			decls = append(decls, Declaration{Name: m[1], Kind: "component", Line: lineNum})
		} else if m := reFuncTS.FindStringSubmatch(line); len(m) > 1 {
			decls = append(decls, Declaration{Name: m[1], Kind: "function", Line: lineNum})
		}
		if m := reClassTS.FindStringSubmatch(line); len(m) > 1 {
			decls = append(decls, Declaration{Name: m[1], Kind: "class", Line: lineNum})
		}
		if m := reInterface.FindStringSubmatch(line); len(m) > 1 {
			decls = append(decls, Declaration{Name: m[1], Kind: "interface", Line: lineNum})
		}
		if m := reTypeAlias.FindStringSubmatch(line); len(m) > 1 {
			decls = append(decls, Declaration{Name: m[1], Kind: "type", Line: lineNum})
		}
		if m := reConstTS.FindStringSubmatch(line); len(m) > 1 {
			if strings.Contains(line, "export") {
				decls = append(decls, Declaration{Name: m[1], Kind: "const", Line: lineNum})
			}
		}
	}

	return dedupDeclarations(decls)
}

func extractPythonDeclarations(content string) []Declaration {
	var decls []Declaration
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1
		if m := reFuncPy.FindStringSubmatch(line); len(m) > 1 {
			decls = append(decls, Declaration{Name: m[1], Kind: "function", Line: lineNum})
		}
		if m := reClassPy.FindStringSubmatch(line); len(m) > 1 {
			decls = append(decls, Declaration{Name: m[1], Kind: "class", Line: lineNum})
		}
	}
	return decls
}

// ---------- Helpers ----------

func countLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n") + 1
}

func isTestFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, "_test.") || strings.HasPrefix(filepath.Base(lower), "test_") ||
		strings.Contains(lower, "__tests__")
}

func readTextFile(root, name string, maxSize int) string {
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return ""
	}
	text := string(data)
	if len(text) > maxSize {
		text = text[:maxSize] + "\n... (truncated)"
	}
	return text
}

func extractMatches(re *regexp.Regexp, content string, group int, max int) []string {
	matches := re.FindAllStringSubmatch(content, -1)
	var results []string
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) > group && m[group] != "" && !seen[m[group]] {
			results = append(results, m[group])
			seen[m[group]] = true
		}
		if len(results) >= max {
			break
		}
	}
	return results
}

func dedupDeclarations(decls []Declaration) []Declaration {
	seen := make(map[string]bool)
	var result []Declaration
	for _, d := range decls {
		key := d.Kind + ":" + d.Name
		if !seen[key] {
			seen[key] = true
			result = append(result, d)
		}
	}
	return result
}

func mergeMaps(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}
