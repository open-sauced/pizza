package cache

import "testing"

func TestOpenAndFetch(t *testing.T) {
	tests := []struct {
		name            string
		cacheDir        string
		repos           []string
		neverEvictRepos map[string]bool
	}{
		{
			name:     "Puts repos into cache in sequential order",
			cacheDir: t.TempDir(),
			repos: []string{
				"https://github.com/open-sauced/pizza",
			},
			neverEvictRepos: map[string]bool{
				"https://github.com/kubernetes/kubernetes": true,
				"https://github.com/open-sauced/pizza-cli": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new LRU cache
			c, err := NewGitRepoLRUCache(tt.cacheDir, 1, tt.neverEvictRepos)
			if err != nil {
				t.Fatalf("unexpected err: %s", err.Error())
			}

			// Populate the cache with the repos
			for _, repo := range tt.repos {
				repoFp, err := c.Put(repo)
				if err != nil {
					t.Fatalf("unexpected err putting to cache: %s", err.Error())
				}
				repoFp.Done()
			}

			// Get the first element in the cache
			repoFp := c.dll.Front().Value.(*GitRepoFilePath)
			repoFp.lock.Lock()
			defer repoFp.Done()

			// Open and fetch the repo ensuring a non-nil git repo is returned
			openedRepo, err := repoFp.OpenAndFetch()
			if openedRepo == nil || err != nil {
				t.Fatalf("Opened repo unexpectedly failed to open and/or fetch: %s", err.Error())
			}
		})
	}
}
