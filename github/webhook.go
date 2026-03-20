package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/aryans1319/devdoctor/analyzer"
	"github.com/aryans1319/devdoctor/config"
	"github.com/aryans1319/devdoctor/models"
)

// PullRequestEvent is the payload GitHub sends when a PR is opened/updated
type PullRequestEvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PullRequest struct {
		Head struct {
			SHA  string `json:"sha"`
			Repo struct {
				FullName string `json:"full_name"`
				Owner    struct {
					Login string `json:"login"`
				} `json:"owner"`
				Name string `json:"name"`
			} `json:"repo"`
		} `json:"head"`
	} `json:"pull_request"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

// WebhookHandler handles incoming GitHub webhook events
type WebhookHandler struct {
	cfg    *config.Config
	gitApp *GitHubApp
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(cfg *config.Config, gitApp *GitHubApp) *WebhookHandler {
	return &WebhookHandler{cfg: cfg, gitApp: gitApp}
}

// Handle is the main HTTP handler for /webhook
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !verifySignature(h.cfg.GitHubWebhookSecret, r.Header.Get("X-Hub-Signature-256"), body) {
		log.Println("invalid webhook signature")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	if r.Header.Get("X-GitHub-Event") != "pull_request" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var event PullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "could not parse payload", http.StatusBadRequest)
		return
	}

	if event.Action != "opened" && event.Action != "synchronize" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Return 200 immediately — process in background
	w.WriteHeader(http.StatusOK)
	go h.processPR(event)
}

// processPR runs the full scan pipeline in a goroutine
func (h *WebhookHandler) processPR(event PullRequestEvent) {
	owner := event.PullRequest.Head.Repo.Owner.Login
	repo := event.PullRequest.Head.Repo.Name
	prNumber := event.Number
	sha := event.PullRequest.Head.SHA
	installationID := event.Installation.ID

	log.Printf("scanning PR #%d in %s/%s", prNumber, owner, repo)

	// Get installation token
	token, err := h.gitApp.GetInstallationToken(installationID)
	if err != nil {
		log.Printf("could not get installation token: %v", err)
		return
	}

	client := NewClient(token)

	// Post pending status immediately
	_ = client.PostCommitStatus(owner, repo, sha, "pending", "DevDoctor is scanning...")

	// Get changed files in this PR
	files, err := client.GetPRFiles(owner, repo, prNumber)
	if err != nil {
		log.Printf("could not get PR files: %v", err)
		_ = client.PostCommitStatus(owner, repo, sha, "error", "DevDoctor failed to fetch files")
		return
	}

	// Filter only files we know how to analyze
	relevantFiles := filterRelevantFiles(files)
	if len(relevantFiles) == 0 {
		log.Printf("no relevant files found in PR #%d", prNumber)
		_ = client.PostCommitStatus(owner, repo, sha, "success", "DevDoctor — no infrastructure files changed")
		return
	}

	// Fetch content and analyze each file using the registry
	var allResults []analyzerResult
	for _, file := range relevantFiles {
		content, err := client.GetFileContent(file.RawURL)
		if err != nil {
			log.Printf("could not fetch %s: %v", file.Filename, err)
			continue
		}

		result := analyzeFileFromPR(file.Filename, []byte(content))
		allResults = append(allResults, result)
	}

	// Post PR comment
	comment := formatComment(allResults)
	if err := client.PostComment(owner, repo, prNumber, comment); err != nil {
		log.Printf("could not post comment: %v", err)
		return
	}

	// Post commit status
	overallScore := calcOverallScore(allResults)
	if overallScore >= 70 {
		_ = client.PostCommitStatus(owner, repo, sha, "success",
			fmt.Sprintf("DevDoctor — %d/100 — No critical issues", overallScore))
	} else {
		_ = client.PostCommitStatus(owner, repo, sha, "failure",
			fmt.Sprintf("DevDoctor — %d/100 — Critical issues found", overallScore))
	}

	log.Printf("scan complete for PR #%d — score: %d/100", prNumber, overallScore)
}

// analyzeFileFromPR uses the registry to analyze a file from PR content
func analyzeFileFromPR(filename string, content []byte) analyzerResult {
	result := analyzerResult{Filename: filename, Score: 100}

	analyzers := analyzer.FindAnalyzers(filename)
	if len(analyzers) == 0 {
		return result
	}

	ctx := models.AnalysisContext{
		FilePath:     filename,
		FileType:     analyzers[0].Name(),
		Content:      content,
		ChangedLines: map[int]bool{},
		IsPRScan:     true,
	}

	var allIssues []models.Issue
	for _, a := range analyzers {
		issues := a.Analyze(ctx)
		allIssues = append(allIssues, issues...)
	}

	result.FileType = ctx.FileType
	result.Score = calcScore(len(allIssues))
	for _, issue := range allIssues {
		result.Issues = append(result.Issues, issueItem{
			Severity:   string(issue.Severity),
			Rule:       issue.Rule,
			Message:    issue.Message,
			Suggestion: issue.Suggestion,
			Line:       issue.Line,
		})
	}

	return result
}

// filterRelevantFiles keeps only files the registry can handle
func filterRelevantFiles(files []PRFile) []PRFile {
	var relevant []PRFile
	for _, f := range files {
		if f.Status == "removed" {
			continue
		}
		if len(analyzer.FindAnalyzers(f.Filename)) > 0 {
			relevant = append(relevant, f)
		}
	}
	return relevant
}

// calcScore is a local wrapper — avoids importing analyzer internals
func calcScore(issueCount int) int {
	score := 100 - (issueCount * 10)
	if score < 0 {
		return 0
	}
	return score
}

// calcOverallScore averages scores across all results
func calcOverallScore(results []analyzerResult) int {
	if len(results) == 0 {
		return 100
	}
	total := 0
	for _, r := range results {
		total += r.Score
	}
	return total / len(results)
}

// verifySignature verifies the HMAC-SHA256 signature from GitHub
func verifySignature(secret, signature string, body []byte) bool {
	if signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// analyzerResult wraps results for the GitHub App layer
type analyzerResult struct {
	Filename  string
	FileType  string
	Score     int
	Issues    []issueItem
	AISummary string
}

type issueItem struct {
	Severity   string
	Rule       string
	Message    string
	Suggestion string
	Line       int
}

// keep existing AnalyzeDockerfileContent and AnalyzeComposeContent
// public functions working for backward compatibility
func analyzeFileContent(filename, content string) analyzerResult {
	return analyzeFileFromPR(filename, []byte(content))
}

// isRelevantFile checks if a file can be analyzed
func isRelevantFile(filename string) bool {
	return len(analyzer.FindAnalyzers(filename)) > 0
}

func extractBaseName(filename string) string {
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		return filename[idx+1:]
	}
	return filename
}