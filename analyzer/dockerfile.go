package analyzer

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/aryans1319/devdoctor/models"
)

func analyzeDockerfile(path string) models.FileResult {
	result := models.FileResult{
		FilePath: path,
		FileType: "Dockerfile",
	}

	file, err := os.Open(path)
	if err != nil {
		return result
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	result.Issues = checkDockerfileRules(lines)

	// Check for .dockerignore
	if issue := checkDockerignore(path); issue != nil {
		result.Issues = append(result.Issues, *issue)
	}

	// Check for partial version pinning
	if issue := checkPartialVersionPin(lines); issue != nil {
		result.Issues = append(result.Issues, *issue)
	}

	result.Score = calculateScore(len(result.Issues))
	return result
}

func checkDockerfileRules(lines []string) []models.Issue {
	var issues []models.Issue

	hasUser := false
	hasHealthcheck := false
	hasLatestTag := false
	runCount := 0
	hasCopyAll := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Check for latest tag
		if strings.HasPrefix(upper, "FROM") && strings.Contains(upper, ":LATEST") {
			hasLatestTag = true
			issues = append(issues, models.Issue{
				Line:     i + 1,
				Severity: models.SeverityError,
				Rule:     "NO_LATEST_TAG",
				Message:  "Base image uses ':latest' tag — builds are not reproducible",
			})
		}

		// Check for unpinned base image
		if strings.HasPrefix(upper, "FROM") && !strings.Contains(trimmed, ":") && !strings.Contains(upper, "AS") {
			if !hasLatestTag {
				issues = append(issues, models.Issue{
					Line:     i + 1,
					Severity: models.SeverityWarning,
					Rule:     "UNPINNED_BASE_IMAGE",
					Message:  "Base image has no version tag — pin to a specific version for reproducibility",
				})
			}
		}

		// Count RUN commands
		if strings.HasPrefix(upper, "RUN") {
			runCount++
		}

		// Check for COPY . .
		if strings.HasPrefix(upper, "COPY") && (strings.Contains(trimmed, "COPY . .") || strings.Contains(trimmed, "COPY ./ .")) {
			hasCopyAll = true
			issues = append(issues, models.Issue{
				Line:     i + 1,
				Severity: models.SeverityWarning,
				Rule:     "COPY_ALL",
				Message:  "'COPY . .' copies everything including sensitive files — use .dockerignore or copy specific files",
			})
		}

		// Check for secrets in ENV or ARG
		if strings.HasPrefix(upper, "ENV") || strings.HasPrefix(upper, "ARG") {
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "password") || strings.Contains(lower, "secret") ||
				strings.Contains(lower, "api_key") || strings.Contains(lower, "token") {
				issues = append(issues, models.Issue{
					Line:     i + 1,
					Severity: models.SeverityError,
					Rule:     "HARDCODED_SECRET",
					Message:  "Possible secret or password hardcoded in ENV/ARG — use Docker secrets or runtime env vars",
				})
			}
		}

		// Check for USER instruction
		if strings.HasPrefix(upper, "USER") {
			hasUser = true
		}

		// Check for HEALTHCHECK
		if strings.HasPrefix(upper, "HEALTHCHECK") {
			hasHealthcheck = true
		}
	}

	// Multiple RUN commands
	if runCount > 3 {
		issues = append(issues, models.Issue{
			Line:     0,
			Severity: models.SeverityWarning,
			Rule:     "MULTIPLE_RUN_COMMANDS",
			Message:  "Multiple RUN commands found — combine them using && to reduce image layers",
		})
	}

	// No USER set
	if !hasUser {
		issues = append(issues, models.Issue{
			Line:     0,
			Severity: models.SeverityWarning,
			Rule:     "NO_USER",
			Message:  "No USER instruction found — container will run as root which is a security risk",
		})
	}

	// No HEALTHCHECK
	if !hasHealthcheck {
		issues = append(issues, models.Issue{
			Line:     0,
			Severity: models.SeverityInfo,
			Rule:     "NO_HEALTHCHECK",
			Message:  "No HEALTHCHECK instruction — Docker won't know if your container is actually healthy",
		})
	}

	_ = hasCopyAll
	return issues
}

func checkDockerignore(dockerfilePath string) *models.Issue {
	dir := filepath.Dir(dockerfilePath)
	dockerignorePath := filepath.Join(dir, ".dockerignore")

	if _, err := os.Stat(dockerignorePath); os.IsNotExist(err) {
		issue := models.Issue{
			Line:     0,
			Severity: models.SeverityWarning,
			Rule:     "NO_DOCKERIGNORE",
			Message:  "No .dockerignore file found — build context may include unnecessary or sensitive files",
		}
		return &issue
	}
	return nil
}

func checkPartialVersionPin(lines []string) *models.Issue {
	vagueVersions := []string{
		"jammy", "focal", "bullseye", "buster",
		"alpine", "slim", "jre", "jdk",
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if !strings.HasPrefix(upper, "FROM") {
			continue
		}

		parts := strings.Fields(trimmed)
		if len(parts) < 2 {
			continue
		}

		image := parts[1]

		// Skip if already fully pinned with sha256
		if strings.Contains(image, "@sha256:") {
			continue
		}

		for _, vague := range vagueVersions {
			if strings.Contains(strings.ToLower(image), vague) && !strings.Contains(image, "@") {
				issue := models.Issue{
					Line:     i + 1,
					Severity: models.SeverityInfo,
					Rule:     "PARTIAL_VERSION_PIN",
					Message:  "Image '" + image + "' uses a partial version tag — consider pinning to a full version or digest for full reproducibility",
				}
				return &issue
			}
		}
	}
	return nil
}

func calculateScore(issueCount int) int {
	score := 100
	score -= issueCount * 10
	if score < 0 {
		return 0
	}
	return score
}