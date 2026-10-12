package gitops

import (
	"testing"
)

func TestCloneRepo_InvalidURL_ReturnsError(t *testing.T) {
	_, err := CloneRepo("https://invalid.example.invalid/repo.git", "main", "")
	if err == nil {
		t.Fatal("expected error for invalid repo URL")
	}
}

func TestCloneRepo_InvalidURL_WithToken_ReturnsError(t *testing.T) {
	_, err := CloneRepo("https://invalid.example.invalid/repo.git", "main", "mytoken")
	if err == nil {
		t.Fatal("expected error for invalid repo URL with token")
	}
}
