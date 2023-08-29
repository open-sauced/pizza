package common

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
)

// IsValidGitRepo returns true if the provided git repo URL is a valid and reachable
// git repository. This is equivalent to running "git ls-remote" on the provided
// URL string. This may result in some unexpected "authentication required" or
// "repository not found" errors which is standard for git to return in these
// situations.
func IsValidGitRepo(repoURL string) (bool, error) {
	remoteConfig := &config.RemoteConfig{
		Name: "source",
		URLs: []string{
			repoURL,
		},
	}

	remote := git.NewRemote(memory.NewStorage(), remoteConfig)

	_, err := remote.List(&git.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("could not list remote repository: %s", err.Error())
	}

	return true, nil
}

// NormalizeGitURL attempts to take a raw git repo URL and ensure it is normalized
// before being validated or entered into the database
func NormalizeGitURL(repoURL string) (string, error) {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}

	// Check if it has a valid protocol specified (e.g., https, ssh, git)
	if parsedURL.Scheme != "git" && parsedURL.Scheme != "https" && parsedURL.Scheme != "file" {
		return "", fmt.Errorf("repo URL missing valid protocol scheme (https, git, file): %s", repoURL)
	}

	// Trim trailing slashes
	// Example: https://github.com/open-sauced/pizza/ to https://github.com/open-sauced/pizza
	trimmedPath := strings.TrimSuffix(parsedURL.Path, "/")

	// Remove .git suffix if present
	// Example: https://github.com/open-sauced/pizza.git to https://github.com/open-sauced/pizza
	trimmedPath = strings.TrimSuffix(trimmedPath, ".git")

	parsedURL.Path = trimmedPath

	return parsedURL.String(), nil
}
