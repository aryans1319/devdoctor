package analyzer

import (
	"strings"

	"github.com/aryans1319/devdoctor/models"
)

// Registry holds all registered analyzers
type Registry struct {
	analyzers []Analyzer
}

// globalRegistry is the single instance used across the app
var globalRegistry = &Registry{}

// Register adds an analyzer to the global registry
func Register(a Analyzer) {
	globalRegistry.analyzers = append(globalRegistry.analyzers, a)
}

// All returns all registered analyzers
func All() []Analyzer {
	return globalRegistry.analyzers
}

// FindAnalyzers returns all analyzers that match the given file path
func FindAnalyzers(filePath string) []Analyzer {
	var matched []Analyzer
	for _, a := range globalRegistry.analyzers {
		if a.Match(filePath) {
			matched = append(matched, a)
		}
	}
	return matched
}

// init registers all built-in analyzers when the package loads
func init() {
	Register(&DockerfileAnalyzer{})
	Register(&ComposeAnalyzer{})
	Register(&KubernetesAnalyzer{})
	Register(&ActionsAnalyzer{})
}

// DockerfileAnalyzer implements Analyzer for Dockerfiles
type DockerfileAnalyzer struct{}

func (d *DockerfileAnalyzer) Name() string { return "Dockerfile" }

func (d *DockerfileAnalyzer) Match(filePath string) bool {
	base := baseName(filePath)
	return base == "dockerfile" || strings.HasPrefix(base, "dockerfile.")
}

func (d *DockerfileAnalyzer) Analyze(ctx models.AnalysisContext) []models.Issue {
	lines := strings.Split(string(ctx.Content), "\n")
	issues := checkDockerfileRules(lines)
	if issue := checkPartialVersionPin(lines); issue != nil {
		issues = append(issues, *issue)
	}
	if issue := checkDockerignoreFromContext(ctx.FilePath); issue != nil {
		issues = append(issues, *issue)
	}
	return issues
}

// ComposeAnalyzer implements Analyzer for docker-compose files
type ComposeAnalyzer struct{}

func (c *ComposeAnalyzer) Name() string { return "docker-compose" }

func (c *ComposeAnalyzer) Match(filePath string) bool {
	lower := strings.ToLower(filePath)
	base := baseName(lower)
	return base == "docker-compose.yml" ||
		base == "docker-compose.yaml" ||
		strings.Contains(lower, "docker-compose")
}

func (c *ComposeAnalyzer) Analyze(ctx models.AnalysisContext) []models.Issue {
	return analyzeComposeContent(ctx.Content)
}

// KubernetesAnalyzer implements Analyzer for Kubernetes manifests
type KubernetesAnalyzer struct{}

func (k *KubernetesAnalyzer) Name() string { return "kubernetes" }

func (k *KubernetesAnalyzer) Match(filePath string) bool {
	return !IsActionsFile(filePath) && IsKubernetesFile(filePath)
}

func (k *KubernetesAnalyzer) Analyze(ctx models.AnalysisContext) []models.Issue {
	return checkKubernetesRules(ctx.Content, ctx.FilePath)
}

// ActionsAnalyzer implements Analyzer for GitHub Actions workflows
type ActionsAnalyzer struct{}

func (a *ActionsAnalyzer) Name() string { return "github-actions" }

func (a *ActionsAnalyzer) Match(filePath string) bool {
	return IsActionsFile(filePath)
}

func (a *ActionsAnalyzer) Analyze(ctx models.AnalysisContext) []models.Issue {
	return checkActionsRules(ctx.Content, ctx.FilePath)
}

// baseName returns the lowercased filename from a full path
func baseName(filePath string) string {
	lower := strings.ToLower(filePath)
	if idx := strings.LastIndex(lower, "/"); idx >= 0 {
		return lower[idx+1:]
	}
	return lower
}