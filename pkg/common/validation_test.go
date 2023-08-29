package common

import "testing"

func TestNormalizeGitURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Fully normalizes",
			url:      "https://github.com/user/repo.git/",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "Removes trailing .git",
			url:      "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "Removes trailing slash",
			url:      "https://github.com/user/repo/",
			expected: "https://github.com/user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizedURL, err := NormalizeGitURL(tt.url)
			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}

			if normalizedURL != tt.expected {
				t.Fatalf("normalized URL: %s is not expected: %s", normalizedURL, tt.expected)
			}
		})
	}
}

func TestNormalizeGitURLError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "Missing protocol fails",
			url:  "github.com/user/repo",
		},
		{
			name: "Malformed protocol fails",
			url:  "ht:/github.com/user/repo",
		},
		{
			name: "Unusable protocol fails",
			url:  "ssh://github.com/user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizedURL, err := NormalizeGitURL(tt.url)
			if err == nil {
				t.Fatalf("expected error, got none: %s", normalizedURL)
			}
		})
	}
}
