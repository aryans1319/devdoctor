package github

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GitHubApp holds the App ID and private key for JWT signing
type GitHubApp struct {
	AppID      int64
	PrivateKey *rsa.PrivateKey
}

// NewGitHubApp loads the private key from file or base64 env var
func NewGitHubApp(appIDStr, privateKeyPath string) (*GitHubApp, error) {
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid app ID: %w", err)
	}

	// Try base64 env var first (used in production on Render)
	// Fall back to file path (used in local development)
	var keyData []byte

	base64Key := os.Getenv("GITHUB_PRIVATE_KEY_BASE64")
	if base64Key != "" {
		keyData, err = base64.StdEncoding.DecodeString(base64Key)
		if err != nil {
			return nil, fmt.Errorf("could not decode base64 private key: %w", err)
		}
	} else {
		keyData, err = os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("could not read private key: %w", err)
		}
	}

	privateKey, err := parsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("could not parse private key: %w", err)
	}

	return &GitHubApp{
		AppID:      appID,
		PrivateKey: privateKey,
	}, nil
}

// GenerateJWT creates a signed JWT valid for 10 minutes
func (g *GitHubApp) GenerateJWT() (string, error) {
	now := time.Now()

	claims := jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": g.AppID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(g.PrivateKey)
}

// GetInstallationToken exchanges a JWT for an installation access token
func (g *GitHubApp) GetInstallationToken(installationID int64) (string, error) {
	jwt, err := g.GenerateJWT()
	if err != nil {
		return "", fmt.Errorf("could not generate JWT: %w", err)
	}

	url := fmt.Sprintf(
		"https://api.github.com/app/installations/%d/access_tokens",
		installationID,
	)

	resp, err := doGitHubPost(url, jwt, nil)
	if err != nil {
		return "", fmt.Errorf("could not get installation token: %w", err)
	}

	token, ok := resp["token"].(string)
	if !ok {
		return "", fmt.Errorf("invalid token response from GitHub")
	}

	return token, nil
}

func parsePrivateKey(keyData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return key, nil
}