package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aryans1319/devdoctor/models"
	"github.com/joho/godotenv"
)

// Gemini API request/response structs
type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// AI suggestions response structure
type aiSuggestions struct {
	Files   []fileSuggestion `json:"files"`
	Summary string           `json:"summary"`
}

type fileSuggestion struct {
	FilePath string          `json:"filePath"`
	Issues   []issueFix      `json:"issues"`
}

type issueFix struct {
	Rule       string `json:"rule"`
	Suggestion string `json:"suggestion"`
}

func enrichWithAI(result models.ScanResult) models.ScanResult {
	_ = godotenv.Load()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("⚠️  GEMINI_API_KEY not set — skipping AI suggestions")
		return result
	}

	prompt := buildPrompt(result)
	responseText, err := callGemini(apiKey, prompt)
	if err != nil {
		fmt.Println("⚠️  AI suggestions failed:", err)
		return result
	}

	return applyAISuggestions(result, responseText)
}

func buildPrompt(result models.ScanResult) string {
	var sb strings.Builder

	sb.WriteString("You are a DevOps expert. Analyze these issues found in a project and provide specific, actionable fixes.\n\n")
	sb.WriteString("Respond ONLY with a JSON object in this exact format, no markdown, no extra text:\n")
	sb.WriteString(`{
  "files": [
    {
      "filePath": "path/to/file",
      "issues": [
        {
          "rule": "RULE_NAME",
          "suggestion": "specific fix here"
        }
      ]
    }
  ],
  "summary": "overall summary of the project health"
}` + "\n\n")

	sb.WriteString("Issues found:\n\n")

	for _, fileResult := range result.Results {
		sb.WriteString(fmt.Sprintf("File: %s (%s)\n", fileResult.FilePath, fileResult.FileType))
		for _, issue := range fileResult.Issues {
			sb.WriteString(fmt.Sprintf("  - [%s] %s: %s\n", issue.Severity, issue.Rule, issue.Message))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func callGemini(apiKey, prompt string) (string, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s",
		apiKey,
	)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(respBytes, &gemResp); err != nil {
		return "", err
	}

	if len(gemResp.Candidates) == 0 || len(gemResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return gemResp.Candidates[0].Content.Parts[0].Text, nil
}

func applyAISuggestions(result models.ScanResult, responseText string) models.ScanResult {
	// Clean response — strip markdown fences if present
	cleaned := strings.TrimSpace(responseText)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var suggestions aiSuggestions
	if err := json.Unmarshal([]byte(cleaned), &suggestions); err != nil {
		fmt.Println("⚠️  Could not parse AI response:", err)
		return result
	}

	// Match AI suggestions back to issues by rule name
	for i, fileResult := range result.Results {
		for _, fileSug := range suggestions.Files {
			if fileSug.FilePath == fileResult.FilePath {
				for j, issue := range fileResult.Issues {
					for _, fix := range fileSug.Issues {
						if fix.Rule == issue.Rule {
							result.Results[i].Issues[j].Suggestion = fix.Suggestion
						}
					}
				}
				result.Results[i].AISummary = fileSug.FilePath
			}
		}
	}

	// Set overall AI summary on first result for display
	if len(result.Results) > 0 {
		result.Results[0].AISummary = suggestions.Summary
	}

	return result
}