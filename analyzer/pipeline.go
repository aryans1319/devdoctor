package analyzer

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/aryans1319/devdoctor/models"
)

// Pipeline orchestrates the full scan — file discovery,
// analyzer routing, concurrent execution, and AI enrichment
type Pipeline struct {
	registry *Registry
}

// NewPipeline creates a pipeline using the global registry
func NewPipeline() *Pipeline {
	return &Pipeline{registry: globalRegistry}
}

// ScanProject walks the project folder and runs all matching
// analyzers concurrently against each discovered file
func (p *Pipeline) ScanProject(projectPath string) (models.ScanResult, error) {
	result := models.ScanResult{
		ProjectPath: projectPath,
	}

	// Discover all relevant files first
	var filePaths []string
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && shouldSkipDir(info.Name()) {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		// Only collect files that at least one analyzer matches
		if len(FindAnalyzers(path)) > 0 {
			filePaths = append(filePaths, path)
		}
		return nil
	})

	if err != nil {
		return result, err
	}

	// Analyze files concurrently
	fileResults := p.analyzeFilesConcurrently(filePaths)
	result.Results = fileResults
	result.TotalIssues = countTotalIssues(result.Results)
	result.OverallScore = calculateOverallScore(result.Results)

	// Enrich with AI suggestions if any issues found
	if result.TotalIssues > 0 {
		result = enrichWithAI(result)
	}

	return result, nil
}

// analyzeFilesConcurrently runs analyzers on all files using a worker pool
func (p *Pipeline) analyzeFilesConcurrently(filePaths []string) []models.FileResult {
	if len(filePaths) == 0 {
		return nil
	}

	type work struct {
		path string
	}

	jobs := make(chan work, len(filePaths))
	resultsCh := make(chan models.FileResult, len(filePaths))

	// Worker count — capped at 5 or number of files, whichever is smaller
	workerCount := 5
	if len(filePaths) < workerCount {
		workerCount = len(filePaths)
	}

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				fileResult := p.analyzeFile(job.path)
				if fileResult != nil {
					resultsCh <- *fileResult
				}
			}
		}()
	}

	// Send jobs
	for _, path := range filePaths {
		jobs <- work{path: path}
	}
	close(jobs)

	// Wait for all workers then close results
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Collect results
	var results []models.FileResult
	for r := range resultsCh {
		results = append(results, r)
	}

	return results
}

// analyzeFile reads a file and runs all matching analyzers against it
func (p *Pipeline) analyzeFile(filePath string) *models.FileResult {
	analyzers := FindAnalyzers(filePath)
	if len(analyzers) == 0 {
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	ctx := models.AnalysisContext{
		FilePath:     filePath,
		Content:      content,
		ChangedLines: map[int]bool{}, // empty = analyze all lines (CLI mode)
		IsPRScan:     false,
	}

	var allIssues []models.Issue
	fileType := analyzers[0].Name()

	for _, a := range analyzers {
		issues := a.Analyze(ctx)
		allIssues = append(allIssues, issues...)
	}

	result := &models.FileResult{
		FilePath: filePath,
		FileType: fileType,
		Issues:   allIssues,
		Score:    calculateScore(len(allIssues)),
	}

	return result
}