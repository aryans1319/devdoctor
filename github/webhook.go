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
	return &WebhookHandler{
		cfg:    cfg,
		gitApp: gitApp,
	}
}

// Handle is the main HTTP handler for /webhook
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Step 1 — Read the raw body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Step 2 — Verify HMAC signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !verifySignature(h.cfg.GitHubWebhookSecret, signature, body) {
		log.Println("invalid webhook signature")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Step 3 — Only handle pull_request events
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType != "pull_request" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Step 4 — Parse the payload
	var event PullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "could not parse payload", http.StatusBadRequest)
		return
	}

	// Step 5 — Only process opened or synchronized PRs
	if event.Action != "opened" && event.Action != "synchronize" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Step 6 — Return 200 immediately, process in background
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

	// Step 1 — Get installation token
	token, err := h.gitApp.GetInstallationToken(installationID)
	if err != nil {
		log.Printf("could not get installation token: %v", err)
		return
	}

	client := NewClient(token)

	// Step 2 — Post pending status immediately
	_ = client.PostCommitStatus(owner, repo, sha, "pending", "DevDoctor is scanning...")

	// Step 3 — Get changed files in this PR
	files, err := client.GetPRFiles(owner, repo, prNumber)
	if err != nil {
		log.Printf("could not get PR files: %v", err)
		_ = client.PostCommitStatus(owner, repo, sha, "error", "DevDoctor failed to fetch files")
		return
	}

	// Step 4 — Filter only files we care about
	relevantFiles := filterRelevantFiles(files)
	if len(relevantFiles) == 0 {
		log.Printf("no relevant files found in PR #%d", prNumber)
		_ = client.PostCommitStatus(owner, repo, sha, "success", "DevDoctor — no infrastructure files changed")
		return
	}

	// Step 5 — Fetch content and analyze each file
	var allResults []analyzerResult
	for _, file := range relevantFiles {
		content, err := client.GetFileContent(file.RawURL)
		if err != nil {
			log.Printf("could not fetch %s: %v", file.Filename, err)
			continue
		}

		result := analyzeFile(file.Filename, content)
		allResults = append(allResults, result)
	}

	// Step 6 — Build and post PR comment
	comment := formatComment(allResults)
	if err := client.PostComment(owner, repo, prNumber, comment); err != nil {
		log.Printf("could not post comment: %v", err)
		return
	}

	// Step 7 — Post final commit status
	overallScore := calculateOverallScore(allResults)
	if overallScore >= 70 {
		_ = client.PostCommitStatus(owner, repo, sha, "success",
			fmt.Sprintf("DevDoctor — %d/100 — No critical issues", overallScore))
	} else {
		_ = client.PostCommitStatus(owner, repo, sha, "failure",
			fmt.Sprintf("DevDoctor — %d/100 — Critical issues found", overallScore))
	}

	log.Printf("scan complete for PR #%d — score: %d/100", prNumber, overallScore)
}

// analyzerResult wraps a FileResult with the original filename
type analyzerResult struct {
	Filename   string
	FileResult interface{}
	Score      int
	Issues     []issueItem
	AISummary  string
}

type issueItem struct {
	Severity   string
	Rule       string
	Message    string
	Suggestion string
	Line       int
}

// analyzeFile routes a file to the right analyzer based on its name
func analyzeFile(filename, content string) analyzerResult {
	lower := strings.ToLower(filename)
	baseName := filename
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		baseName = filename[idx+1:]
	}
	lowerBase := strings.ToLower(baseName)

	result := analyzerResult{Filename: filename}

	if lowerBase == "dockerfile" || strings.HasPrefix(lowerBase, "dockerfile.") {
		fileResult := analyzer.AnalyzeDockerfileContent(content, filename)
		result.Score = fileResult.Score
		result.AISummary = fileResult.AISummary
		for _, issue := range fileResult.Issues {
			result.Issues = append(result.Issues, issueItem{
				Severity:   string(issue.Severity),
				Rule:       issue.Rule,
				Message:    issue.Message,
				Suggestion: issue.Suggestion,
				Line:       issue.Line,
			})
		}
	} else if lowerBase == "docker-compose.yml" || lowerBase == "docker-compose.yaml" || strings.Contains(lower, "docker-compose") {
		fileResult := analyzer.AnalyzeComposeContent(content, filename)
		result.Score = fileResult.Score
		result.AISummary = fileResult.AISummary
		for _, issue := range fileResult.Issues {
			result.Issues = append(result.Issues, issueItem{
				Severity:   string(issue.Severity),
				Rule:       issue.Rule,
				Message:    issue.Message,
				Suggestion: issue.Suggestion,
				Line:       issue.Line,
			})
		}
	} else {
		result.Score = 100
	}

	return result
}

// filterRelevantFiles keeps only files we know how to analyze
func filterRelevantFiles(files []PRFile) []PRFile {
	var relevant []PRFile
	for _, f := range files {
		if f.Status == "removed" {
			continue
		}
		lower := strings.ToLower(f.Filename)
		baseName := f.Filename
		if idx := strings.LastIndex(f.Filename, "/"); idx >= 0 {
			baseName = f.Filename[idx+1:]
		}
		lowerBase := strings.ToLower(baseName)

		if lowerBase == "dockerfile" ||
			strings.HasPrefix(lowerBase, "dockerfile.") ||
			lowerBase == "docker-compose.yml" ||
			lowerBase == "docker-compose.yaml" ||
			strings.Contains(lower, "docker-compose") {
			relevant = append(relevant, f)
		}
	}
	return relevant
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

func calculateOverallScore(results []analyzerResult) int {
	if len(results) == 0 {
		return 100
	}
	total := 0
	for _, r := range results {
		total += r.Score
	}
	return total / len(results)
}