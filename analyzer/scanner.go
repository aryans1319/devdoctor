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

	// Walk every file and folder inside the project path
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip files we can't read
		}

		// Skip hidden folders like .git, .idea, node_modules
		if info.IsDir() && shouldSkipDir(info.Name()) {
			return filepath.SkipDir
		}

		// Skip directories themselves, only process files
		if info.IsDir() {
			return nil
		}

		fileName := info.Name()

		// Route each file to the right analyzer
		if fileName == "Dockerfile" || strings.HasPrefix(fileName, "Dockerfile.") {
			fileResult := analyzeDockerfile(path)
			result.Results = append(result.Results, fileResult)
		}

		if fileName == "docker-compose.yml" || fileName == "docker-compose.yaml" {
			fileResult := analyzeCompose(path)
			result.Results = append(result.Results, fileResult)
		}

		return nil
	})

	if err != nil {
		return result, err
	}

	// Calculate totals
	result.TotalIssues = countTotalIssues(result.Results)
	result.OverallScore = calculateOverallScore(result.Results)

	// Get AI suggestions for all issues found
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