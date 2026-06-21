package services

import (
	"os"
	"testing"
)

func TestNewOAuthService(t *testing.T) {
	// Set test environment variables
	os.Setenv("GOOGLE_CLIENT_ID", "test-google-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-google-secret")
	os.Setenv("GITHUB_CLIENT_ID", "test-github-id")
	os.Setenv("GITHUB_CLIENT_SECRET", "test-github-secret")
	defer func() {
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")
		os.Unsetenv("GITHUB_CLIENT_ID")
		os.Unsetenv("GITHUB_CLIENT_SECRET")
	}()

	svc := NewOAuthService()

	if svc.GoogleConfig.ClientID != "test-google-id" {
		t.Errorf("Expected Google ClientID 'test-google-id', got '%s'", svc.GoogleConfig.ClientID)
	}

	if svc.GitHubConfig.ClientID != "test-github-id" {
		t.Errorf("Expected GitHub ClientID 'test-github-id', got '%s'", svc.GitHubConfig.ClientID)
	}
}

func TestGetGoogleAuthURL(t *testing.T) {
	svc := NewOAuthService()
	url := svc.GetGoogleAuthURL("test-state")
	if url == "" {
		t.Fatal("Expected non-empty URL")
	}
}

func TestGetGitHubAuthURL(t *testing.T) {
	svc := NewOAuthService()
	url := svc.GetGitHubAuthURL("test-state")
	if url == "" {
		t.Fatal("Expected non-empty URL")
	}
}
