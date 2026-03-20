package analyzer

import "github.com/aryans1319/devdoctor/models"

// ScanProject is the main entry point for CLI scans.
// It delegates to the Pipeline which handles file discovery,
// concurrent analysis, and AI enrichment.
func ScanProject(projectPath string) (models.ScanResult, error) {
	pipeline := NewPipeline()
	return pipeline.ScanProject(projectPath)
}

// shouldSkipDir returns true for directories that should
// never be scanned — version control, build artifacts, dependencies
func shouldSkipDir(name string) bool {
	skipDirs := []string{
		".git", ".idea", ".vscode",
		"node_modules", "vendor", "target",
		".mvn", "dist", "build",
	}
	for _, skip := range skipDirs {
		if name == skip {
			return true
		}
	}
	return false
}