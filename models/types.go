package models

// Severity level of an issue
type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// Issue represents a single problem found in a file
type Issue struct {
	Line        int
	Severity    Severity
	Rule        string
	Message     string
	Suggestion  string // AI-generated fix
	InDiff      bool   // true if this line was part of the PR diff
	DiffPosition int   // position in the diff for inline annotations
}

// FileResult is the result of analyzing one file
type FileResult struct {
	FilePath  string
	FileType  string
	Issues    []Issue
	Score     int
	AISummary string
}

// ScanResult is the final output of a full project scan
type ScanResult struct {
	ProjectPath  string
	Results      []FileResult
	OverallScore int
	TotalIssues  int
}