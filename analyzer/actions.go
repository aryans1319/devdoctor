package analyzer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/aryans1319/devdoctor/models"
	"gopkg.in/yaml.v3"
)

// GitHub Actions workflow structs
type actionsWorkflow struct {
	Name string                    `yaml:"name"`
	On   interface{}               `yaml:"on"`
	Jobs map[string]actionsJob     `yaml:"jobs"`
}

type actionsJob struct {
	Name    string         `yaml:"name"`
	RunsOn  interface{}    `yaml:"runs-on"`
	Timeout *int           `yaml:"timeout-minutes"`
	Steps   []actionsStep  `yaml:"steps"`
	Permissions interface{} `yaml:"permissions"`
}

type actionsStep struct {
	Name string `yaml:"name"`
	Uses string `yaml:"uses"`
	Run  string `yaml:"run"`
	With map[string]interface{} `yaml:"with"`
	Env  map[string]interface{} `yaml:"env"`
}

func analyzeActions(path string) models.FileResult {
	result := models.FileResult{
		FilePath: path,
		FileType: "github-actions",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}

	result.Issues = checkActionsRules(data, path)
	result.Score = calculateScore(len(result.Issues))
	return result
}

func checkActionsRules(data []byte, filePath string) []models.Issue {
	var issues []models.Issue

	var workflow actionsWorkflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		issues = append(issues, models.Issue{
			Severity: models.SeverityError,
			Rule:     "ACTIONS_INVALID_YAML",
			Message:  "GitHub Actions workflow is not valid YAML",
		})
		return issues
	}

	for jobName, job := range workflow.Jobs {

		// Check for missing timeout
		if job.Timeout == nil {
			issues = append(issues, models.Issue{
				Severity: models.SeverityWarning,
				Rule:     "ACTIONS_NO_TIMEOUT",
				Message:  "Job '" + jobName + "' has no timeout-minutes — runaway jobs can block your runner for hours",
			})
		}

		// Check for unpinned runner
		runsOn := normalizeRunsOn(job.RunsOn)
		if strings.HasSuffix(runsOn, "-latest") {
			issues = append(issues, models.Issue{
				Severity: models.SeverityInfo,
				Rule:     "ACTIONS_UNPINNED_RUNNER",
				Message:  "Job '" + jobName + "' uses '" + runsOn + "' — pin to a specific runner version like ubuntu-22.04 for reproducibility",
			})
		}

		for _, step := range job.Steps {
			stepIssues := checkActionStepRules(jobName, step)
			issues = append(issues, stepIssues...)
		}
	}

	// Check for dangerous trigger combinations
	triggerIssues := checkTriggerRules(workflow)
	issues = append(issues, triggerIssues...)

	return issues
}

func checkActionStepRules(jobName string, step actionsStep) []models.Issue {
	var issues []models.Issue

	stepRef := "step"
	if step.Name != "" {
		stepRef = "step '" + step.Name + "'"
	}

	// Check for unpinned action version
	if step.Uses != "" {
		if isUnpinnedAction(step.Uses) {
			issues = append(issues, models.Issue{
				Severity: models.SeverityError,
				Rule:     "ACTIONS_UNPINNED_ACTION",
				Message:  "Job '" + jobName + "' " + stepRef + " uses unpinned action '" + step.Uses + "' — pin to a specific commit SHA or version tag (e.g. actions/checkout@v4)",
			})
		}
	}

	// Check for secrets printed in run steps
	if step.Run != "" {
		runLower := strings.ToLower(step.Run)
		if (strings.Contains(runLower, "echo") || strings.Contains(runLower, "print")) &&
			strings.Contains(step.Run, "secrets.") {
			issues = append(issues, models.Issue{
				Severity: models.SeverityError,
				Rule:     "ACTIONS_SECRET_LEAK",
				Message:  "Job '" + jobName + "' " + stepRef + " may print a secret to logs — never echo secrets in run steps",
			})
		}

		// Check for curl piped to shell — common supply chain attack vector
		if strings.Contains(runLower, "curl") &&
			(strings.Contains(runLower, "| bash") || strings.Contains(runLower, "| sh")) {
			issues = append(issues, models.Issue{
				Severity: models.SeverityWarning,
				Rule:     "ACTIONS_CURL_PIPE_SHELL",
				Message:  "Job '" + jobName + "' " + stepRef + " pipes curl output directly to shell — supply chain risk, download and verify first",
			})
		}
	}

	// Check for hardcoded secrets in env
	for key, val := range step.Env {
		valStr, ok := val.(string)
		if !ok {
			continue
		}
		// Flag raw values that look like secrets (not ${{ secrets.X }} references)
		if isSecretKey(key) && !strings.Contains(valStr, "secrets.") && valStr != "" {
			issues = append(issues, models.Issue{
				Severity: models.SeverityError,
				Rule:     "ACTIONS_HARDCODED_SECRET",
				Message:  "Job '" + jobName + "' " + stepRef + " has possible hardcoded secret in env: " + key,
			})
		}
	}

	return issues
}

func checkTriggerRules(workflow actionsWorkflow) []models.Issue {
	var issues []models.Issue

	// Detect pull_request_target with dangerous permissions
	// pull_request_target runs with write permissions and has access to secrets
	// This is safe for read-only operations but dangerous if it checks out PR code
	onMap, ok := workflow.On.(map[string]interface{})
	if !ok {
		return issues
	}

	if _, hasPRT := onMap["pull_request_target"]; hasPRT {
		issues = append(issues, models.Issue{
			Severity: models.SeverityWarning,
			Rule:     "ACTIONS_PULL_REQUEST_TARGET",
			Message:  "Workflow uses 'pull_request_target' trigger — this runs with write permissions and access to secrets, which can be exploited if PR code is checked out",
		})
	}

	return issues
}

func isUnpinnedAction(uses string) bool {
	// Skip local actions
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "../") {
		return false
	}
	// Skip docker actions
	if strings.HasPrefix(uses, "docker://") {
		return false
	}

	parts := strings.Split(uses, "@")
	if len(parts) != 2 {
		// No @ at all — completely unpinned
		return true
	}

	ref := parts[1]

	// Full SHA commit pin — safest option
	if len(ref) == 40 && isHexString(ref) {
		return false
	}

	// Pinned to a specific version tag like v3, v3.1, v3.1.2 — acceptable
	if strings.HasPrefix(ref, "v") {
		return false
	}

	// Pinned to branch name like main, master — unpinned
	return true
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func normalizeRunsOn(runsOn interface{}) string {
	switch v := runsOn.(type) {
	case string:
		return v
	case []interface{}:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

// AnalyzeActionsContent analyzes a GitHub Actions workflow from raw string content
// Used by the GitHub App which fetches file content from GitHub API
func AnalyzeActionsContent(content, filePath string) models.FileResult {
	result := models.FileResult{
		FilePath: filePath,
		FileType: "github-actions",
	}

	result.Issues = checkActionsRules([]byte(content), filePath)
	result.Score = calculateScore(len(result.Issues))
	return result
}

func IsActionsFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	// Must be YAML
	if !strings.HasSuffix(lower, ".yml") && !strings.HasSuffix(lower, ".yaml") {
		return false
	}
	// Must be inside .github/workflows/
	return strings.Contains(lower, ".github/workflows/") ||
		strings.Contains(lower, ".github"+string(filepath.Separator)+"workflows"+string(filepath.Separator))
}