package analyzer

import "github.com/aryans1319/devdoctor/models"
// calculateScore returns a health score from 0-100
// based on the number of issues found.
// Each issue deducts 10 points from a perfect score.
func calculateScore(issueCount int) int {
	score := 100 - (issueCount * 10)
	if score < 0 {
		return 0
	}
	return score
}

// calculateOverallScore averages scores across all file results
func calculateOverallScore(results []models.FileResult) int {
	if len(results) == 0 {
		return 100
	}
	total := 0
	for _, r := range results {
		total += r.Score
	}
	return total / len(results)
}

// countTotalIssues sums issues across all file results
func countTotalIssues(results []models.FileResult) int {
	total := 0
	for _, r := range results {
		total += len(r.Issues)
	}
	return total
}