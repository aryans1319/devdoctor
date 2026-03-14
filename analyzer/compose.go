package analyzer

import (
	"os"
	"strings"

	"github.com/aryans1319/devdoctor/models"
	"gopkg.in/yaml.v3"
)

// mirrors the structure of a docker-compose.yml file
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image       string            `yaml:"image"`
	Environment interface{}       `yaml:"environment"`
	Ports       []string          `yaml:"ports"`
	Healthcheck *healthcheck      `yaml:"healthcheck"`
	Volumes     []string          `yaml:"volumes"`
}

type healthcheck struct {
	Test []string `yaml:"test"`
}

func analyzeCompose(path string) models.FileResult {
	result := models.FileResult{
		FilePath: path,
		FileType: "docker-compose",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		result.Issues = append(result.Issues, models.Issue{
			Severity: models.SeverityError,
			Rule:     "INVALID_YAML",
			Message:  "docker-compose.yml is not valid YAML",
		})
		return result
	}

	result.Issues = checkComposeRules(compose)
	result.Score = calculateScore(len(result.Issues))
	return result
}

func checkComposeRules(compose composeFile) []models.Issue {
	var issues []models.Issue

	for serviceName, service := range compose.Services {

		// Check for latest tag or no tag
		if service.Image != "" {
			if strings.HasSuffix(service.Image, ":latest") {
				issues = append(issues, models.Issue{
					Severity: models.SeverityError,
					Rule:     "LATEST_TAG",
					Message:  "Service '" + serviceName + "' uses ':latest' tag — pin to a specific version",
				})
			} else if !strings.Contains(service.Image, ":") {
				issues = append(issues, models.Issue{
					Severity: models.SeverityWarning,
					Rule:     "UNPINNED_IMAGE",
					Message:  "Service '" + serviceName + "' has no version tag on image",
				})
			}
		}

		// Check for missing healthcheck
		if service.Healthcheck == nil {
			issues = append(issues, models.Issue{
				Severity: models.SeverityWarning,
				Rule:     "NO_HEALTHCHECK",
				Message:  "Service '" + serviceName + "' has no healthcheck defined",
			})
		}

		// Check for hardcoded secrets in environment
		switch env := service.Environment.(type) {
		case map[string]interface{}:
			for key, val := range env {
				if isSecretKey(key) && val != nil && val != "" {
					issues = append(issues, models.Issue{
						Severity: models.SeverityError,
						Rule:     "HARDCODED_SECRET",
						Message:  "Service '" + serviceName + "' has possible hardcoded secret in environment: " + key,
					})
				}
			}
		case []interface{}:
			for _, item := range env {
				str, ok := item.(string)
				if !ok {
					continue
				}
				parts := strings.SplitN(str, "=", 2)
				if len(parts) == 2 && isSecretKey(parts[0]) && parts[1] != "" {
					issues = append(issues, models.Issue{
						Severity: models.SeverityError,
						Rule:     "HARDCODED_SECRET",
						Message:  "Service '" + serviceName + "' has possible hardcoded secret: " + parts[0],
					})
				}
			}
		}

		// Check for host port binding to 0.0.0.0
		for _, port := range service.Ports {
			if strings.HasPrefix(port, "0.0.0.0") {
				issues = append(issues, models.Issue{
					Severity: models.SeverityWarning,
					Rule:     "OPEN_PORT_BINDING",
					Message:  "Service '" + serviceName + "' binds to 0.0.0.0 — exposes port on all interfaces",
				})
			}
		}
	}

	return issues
}

func isSecretKey(key string) bool {
	lower := strings.ToLower(key)
	keywords := []string{"password", "secret", "api_key", "token", "private_key", "access_key"}
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}