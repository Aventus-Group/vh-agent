// Package analyzesvc implements deep project analysis for VibHost containers.
// Ported from vibhost-mcp/analyze.go — same JSON schema, now served via gRPC.
package analyzesvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Aventus-Group/vh-agent/gen/agentpb"
)

// Service implements the AnalyzeProject gRPC handler.
type Service struct{}

// New returns a ready-to-use analyze service.
func New() *Service { return &Service{} }

// AnalyzeProject performs comprehensive project analysis and returns JSON.
func (s *Service) AnalyzeProject(_ context.Context, req *agentpb.AnalyzeProjectRequest) (*agentpb.AnalyzeProjectResponse, error) {
	path := req.Path
	if path == "" {
		path = "/home/appuser"
	}

	projectPath := findProjectRoot(path)

	analysis := ProjectAnalysis{
		ProjectPath: projectPath,
	}

	// Phase 1: Walk file tree
	walkProject(projectPath, &analysis)

	// Phase 2: Read package.json and detect stack
	analysis.PackageInfo = readPackageJSON(projectPath)
	analysis.Stack = detectStack(projectPath, analysis.PackageInfo)

	// Phase 3: Read config files
	analysis.Configs = readConfigs(projectPath, analysis.FileTree)

	// Phase 4: Detect entry points
	analysis.EntryPoints = detectEntryPoints(projectPath)

	// Phase 5: Detect routes
	analysis.Routes = detectRoutes(projectPath)

	// Phase 6: Analyze source files
	analysis.SourceFiles = analyzeSourceFiles(projectPath, analysis.FileTree)

	// Phase 7: Read README
	analysis.Readme = readTextFile(projectPath, "README.md", 20_000)

	// Phase 8: Read existing CLAUDE.md (for history preservation)
	analysis.ClaudeMD = readTextFile(projectPath, "CLAUDE.md", 50_000)

	data, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("marshal analysis: %w", err)
	}

	return &agentpb.AnalyzeProjectResponse{AnalysisJson: data}, nil
}
