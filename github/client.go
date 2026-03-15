package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PRFile represents a file changed in a pull request
type PRFile struct {
	Filename string `json:"filename"`
	Status   string `json:"status"` // added, modified, removed
	RawURL   string `json:"raw_url"`
}

// Client handles all GitHub API calls
type Client struct {
	token string
}

// NewClient creates a GitHub API client with an installation token
func NewClient(token string) *Client {
	return &Client{token: token}
}

// GetPRFiles returns all files changed in a pull request
func (c *Client) GetPRFiles(owner, repo string, prNumber int) ([]PRFile, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls/%d/files",
		owner, repo, prNumber,
	)

	body, err := c.doGet(url)
	if err != nil {
		return nil, fmt.Errorf("could not fetch PR files: %w", err)
	}

	var files []PRFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("could not parse PR files: %w", err)
	}

	return files, nil
}

// GetFileContent fetches the raw content of a file from GitHub
func (c *Client) GetFileContent(rawURL string) (string, error) {
	body, err := c.doGet(rawURL)
	if err != nil {
		return "", fmt.Errorf("could not fetch file content: %w", err)
	}
	return string(body), nil
}

// PostComment posts a comment on a pull request
func (c *Client) PostComment(owner, repo string, prNumber int, body string) error {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/issues/%d/comments",
		owner, repo, prNumber,
	)

	payload := map[string]string{"body": body}
	_, err := c.doPost(url, payload)
	return err
}

// PostCommitStatus posts a commit status (the check mark next to a commit)
func (c *Client) PostCommitStatus(owner, repo, sha, state, description string) error {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/statuses/%s",
		owner, repo, sha,
	)

	payload := map[string]string{
		"state":       state, // "success", "failure", "pending", "error"
		"description": description,
		"context":     "DevDoctor",
	}

	_, err := c.doPost(url, payload)
	return err
}

// doGet makes an authenticated GET request to GitHub API
func (c *Client) doGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// doPost makes an authenticated POST request to GitHub API
func (c *Client) doPost(url string, payload interface{}) (map[string]interface{}, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s — %s", resp.Status, string(body))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

// doGitHubPost is used by auth.go for unauthenticated app-level POST calls
func doGitHubPost(url, jwt string, payload interface{}) (map[string]interface{}, error) {
	var bodyBytes []byte
	var err error

	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s — %s", resp.Status, string(body))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
}