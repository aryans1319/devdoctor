package models

// Severity level of an issue
type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// A single issue found in a file
type Issue struct {
	Line       int
	Severity   Severity
	Rule       string
	Message    string
	Suggestion string // AI-generated fix
}

// Result of analyzing one file
type FileResult struct {
	FilePath   string
	FileType   string // "Dockerfile", "docker-compose", "kubernetes"
	Issues     []Issue
	Score      int
	AISummary  string
}

// Final output of the entire scan
type ScanResult struct {
	ProjectPath string
	Results     []FileResult
	OverallScore int
	TotalIssues  int
}