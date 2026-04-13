package analyzesvc

// ---------- Data Structures (same JSON schema as vibhost-mcp) ----------

// ProjectAnalysis is the top-level result returned as JSON.
type ProjectAnalysis struct {
	ProjectPath string       `json:"projectPath"`
	Stack       StackInfo    `json:"stack"`
	PackageInfo *PackageJSON `json:"packageInfo,omitempty"`
	FileTree    []FileEntry  `json:"fileTree"`
	Configs     []ConfigFile `json:"configs"`
	SourceFiles []SourceFile `json:"sourceFiles"`
	Routes      []string     `json:"routes,omitempty"`
	EntryPoints []string     `json:"entryPoints,omitempty"`
	Readme      string       `json:"readme,omitempty"`
	ClaudeMD    string       `json:"existingClaudeMd,omitempty"`
	Stats       ProjectStats `json:"stats"`
}

type StackInfo struct {
	Framework      string `json:"framework"`
	Language       string `json:"language"`
	PackageManager string `json:"packageManager"`
	Runtime        string `json:"runtime"`
}

type PackageJSON struct {
	Name            string            `json:"name,omitempty"`
	Version         string            `json:"version,omitempty"`
	Description     string            `json:"description,omitempty"`
	Scripts         map[string]string `json:"scripts,omitempty"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
}

type FileEntry struct {
	Path  string `json:"path"`
	Type  string `json:"type"` // "file" or "dir"
	Size  int64  `json:"size"`
	Lines int    `json:"lines,omitempty"`
}

type ConfigFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type SourceFile struct {
	Path         string        `json:"path"`
	Lines        int           `json:"lines"`
	Imports      []string      `json:"imports,omitempty"`
	Exports      []string      `json:"exports,omitempty"`
	Declarations []Declaration `json:"declarations,omitempty"`
}

type Declaration struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // "function", "class", "component", "const", "interface", "type"
	Line int    `json:"line"`
}

type ProjectStats struct {
	TotalFiles       int   `json:"totalFiles"`
	TotalDirs        int   `json:"totalDirs"`
	TotalLines       int   `json:"totalLines"`
	TotalSizeBytes   int64 `json:"totalSizeBytes"`
	SourceFiles      int   `json:"sourceFiles"`
	ConfigFiles      int   `json:"configFiles"`
	TestFiles        int   `json:"testFiles"`
	MaxDepth         int   `json:"maxDepth"`
	HasDockerfile    bool  `json:"hasDockerfile"`
	HasDockerCompose bool  `json:"hasDockerCompose"`
	HasNginxConfig   bool  `json:"hasNginxConfig"`
	HasEnvFile       bool  `json:"hasEnvFile"`
}
