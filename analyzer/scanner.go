package analyzer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/aryans1319/devdoctor/models"
)

// ScanProject walks the project folder, finds all
// relevant files and sends them to the right analyzer
func ScanProject(projectPath string) (models.ScanResult, error) {
	result := models.ScanResult{
		ProjectPath: projectPath,
	}

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && shouldSkipDir(info.Name()) {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		fileName := info.Name()

		// Dockerfile
		if fileName == "Dockerfile" || strings.HasPrefix(fileName, "Dockerfile.") {
			result.Results = append(result.Results, analyzeDockerfile(path))
		}

		// docker-compose
		if fileName == "docker-compose.yml" || fileName == "docker-compose.yaml" {
			result.Results = append(result.Results, analyzeCompose(path))
		}

		// GitHub Actions
		if IsActionsFile(path) {
			result.Results = append(result.Results, analyzeActions(path))
		}

		// Kubernetes — must come after Actions check since both are YAML
		if !IsActionsFile(path) && IsKubernetesFile(path) {
			result.Results = append(result.Results, analyzeKubernetes(path))
		}

		return nil
	})

	if err != nil {
		return result, err
	}

	result.TotalIssues = countTotalIssues(result.Results)
	result.OverallScore = calculateOverallScore(result.Results)

	if result.TotalIssues > 0 {
		result = enrichWithAI(result)
	}

	return result, nil
}

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

func countTotalIssues(results []models.FileResult) int {
	total := 0
	for _, r := range results {
		total += len(r.Issues)
	}
	return total
}

func calculateOverallScore(results []models.FileResult) int {
	if len(results) == 0 {
		return 100
	}
	total := 0
	for _, r := range results {
		total += r.Score
	}
	return total / len(results)
}