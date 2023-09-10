package clients

import (
	"fmt"
	"testing"

	"github.com/google/go-github/v54/github"
)

func createRepoList(org string, totalCount int, archiveCount int) []*github.Repository {
	var repoList []*github.Repository
	for i := 0; i < totalCount; i++ {
		var archiveVal = (i < archiveCount)
		var htmlURL = fmt.Sprintf("https://github.com/%s/repo-%d", org, i)
		repoList = append(repoList, &github.Repository{
			Archived: &archiveVal,
			HTMLURL:  &htmlURL,
		})
	}
	return repoList
}

func TestFilterGithubArchivedRepos(t *testing.T) {
	totalCount := 10
	archiveCount := 2
	filteredCountExpected := (totalCount - archiveCount)
	originalRepoList := createRepoList("open-sauced", totalCount, archiveCount)
	filteredRepoList := FilterGithubArchivedRepos(originalRepoList)
	if len(filteredRepoList) != filteredCountExpected {
		t.Errorf("FilteredArchivedRepos() should yield %d items; got %d", filteredCountExpected, len(filteredRepoList))
	}
}

func TestGetGithubRepoHTMLUrls(t *testing.T) {
	expected := []string{
		"https://github.com/open-sauced/repo-0",
		"https://github.com/open-sauced/repo-1",
		"https://github.com/open-sauced/repo-2",
	}
	repos := createRepoList("open-sauced", 3, 0)
	got := GetGithubRepoHTMLUrls(repos)
	if len(expected) != len(got) {
		t.Errorf("GetRepoHTMLUrls() should yield count matching input")
	}
	for i := 0; i < len(got); i++ {
		if got[i] != expected[i] {
			t.Errorf(`Expected GetRepoHTMLUrls()[%d] to yield "%s"; got "%s"`, i, expected[i], got[i])
		}
	}
}
