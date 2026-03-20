package analyzer

import (
	"github.com/aryans1319/devdoctor/models"
)

// Analyzer is the interface every analyzer must implement.
// Adding a new analyzer means creating one struct that
// implements this interface and registering it — nothing else changes.
type Analyzer interface {
	// Name returns the human-readable name of the analyzer
	// e.g. "Dockerfile", "Kubernetes", "GitHub Actions"
	Name() string

	// Match returns true if this analyzer can handle the given file path
	Match(filePath string) bool

	// Analyze runs the analysis and returns a list of issues found
	Analyze(ctx models.AnalysisContext) []models.Issue
}