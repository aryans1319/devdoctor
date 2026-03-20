package models

// AnalysisContext carries all information an analyzer needs
// to analyze a file — works for both CLI and GitHub App scans
type AnalysisContext struct {
	// FilePath is the path to the file being analyzed
	FilePath string

	// FileType is the detected type e.g. "Dockerfile", "kubernetes"
	FileType string

	// Content is the raw file content
	Content []byte

	// ChangedLines contains line numbers that were modified in a PR diff
	// Empty map means analyze all lines (CLI mode)
	ChangedLines map[int]bool

	// IsPRScan is true when scanning from a GitHub App PR webhook
	IsPRScan bool
}

// IsLineChanged returns true if the given line was changed in the PR diff
// In CLI mode (ChangedLines is empty) all lines are considered changed
func (ctx *AnalysisContext) IsLineChanged(line int) bool {
	if len(ctx.ChangedLines) == 0 {
		return true
	}
	return ctx.ChangedLines[line]
}