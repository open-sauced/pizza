package cache

import (
	"os"
	"sync"
	"testing"
)

// These tests require at least 1 Gb free disk space to work correctly.
//
// Each call to NewGitRepoLRUCache uses "1" as the minimum amount of free
// disk space before the LRU cache automatically begins evicting elements.
// A configured minimum free disk that is _more_ than the _actual_ size of
// the disk itself (example: min Free Disk = 100Gb, actual size of disk = 25Gb)
// will result in the LRU cache only ever having 1 element, the last "Put" element.

// validateCache is a convinence method for testing that validates a given cache
func validateCache(t *testing.T, c *GitRepoLRUCache, expected []string) {
	if len(c.hm) != len(expected) {
		t.Fatalf("cache hashmap not the expected size: %d, %d", len(c.hm), len(expected))
	}

	if c.dll.Len() != len(expected) {
		t.Fatalf("cache doubly linked list not the expected size: %d, %d", c.dll.Len(), len(expected))
	}

	node := c.dll.Front()
	i := 0

	for node != nil {
		if node.Value.(*GitRepoFilePath).key != expected[i] {
			t.Fatalf("GitRepoFilePath and expected path are not the same: %s, %s", node.Value.(*GitRepoFilePath).key, expected[i])
		}

		_, err := os.Stat(node.Value.(*GitRepoFilePath).path)
		if err != nil {
			t.Fatalf("unexpected err on checking if cloned repo present: %s", err.Error())
		}

		node = node.Next()
		i++
	}
}

func TestNewGitRepoLRUCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		cacheDir        string
		wantErr         bool
		neverEvictRepos map[string]bool
	}{
		{
			name:     "Default case",
			cacheDir: t.TempDir(),
			wantErr:  false,
			neverEvictRepos: map[string]bool{
				"no-test": true,
			},
		},
		{
			name:     "Fails when directory doesn't exist",
			cacheDir: "/should/not/exist",
			wantErr:  true,
			neverEvictRepos: map[string]bool{
				"no-test": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewGitRepoLRUCache(tt.cacheDir, 1, tt.neverEvictRepos)
			if tt.wantErr && err != nil {
				return
			}

			if tt.wantErr && err == nil {
				t.Fatalf("expected error but got: %s", err.Error())
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected err: %s", err.Error())
			}

			if c.dir != tt.cacheDir {
				t.Fatalf("unexpected cache dir found. Expected: %s. Actual: %s.", c.dir, tt.cacheDir)
			}

			if len(c.hm) != 0 {
				t.Fatalf("expected cache hashmap length to be 0 for new cache. Actual: %d.", len(c.hm))
			}

			if c.dll.Len() != 0 {
				t.Fatalf("expected cache doubly linked list length to be 0 for new cache. Actual: %d.", c.dll.Len())
			}
		})
	}
}

func TestPutGitRepoLRUCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		cacheDir              string
		repos                 []string
		expectedCacheOrdering []string
		neverEvictRepos       map[string]bool
	}{
		{
			name:     "Puts repos into cache in sequential order",
			cacheDir: t.TempDir(),
			repos: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/pizza-cli",
				"https://github.com/open-sauced/insights",
			},
			expectedCacheOrdering: []string{
				"https://github.com/open-sauced/insights",
				"https://github.com/open-sauced/pizza-cli",
				"https://github.com/open-sauced/pizza",
			},
			neverEvictRepos: map[string]bool{
				"no-test": true,
			},
		},
		{
			name:     "Most recently used is first in order",
			cacheDir: t.TempDir(),
			repos: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/pizza-cli",
				"https://github.com/open-sauced/insights",
				// Note this repo is "Put" last and should appear first in the cache
				"https://github.com/open-sauced/pizza",
			},
			expectedCacheOrdering: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/insights",
				"https://github.com/open-sauced/pizza-cli",
			},
			neverEvictRepos: map[string]bool{
				"no-test": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewGitRepoLRUCache(tt.cacheDir, 1, tt.neverEvictRepos)
			if err != nil {
				t.Fatalf("unexpected err: %s", err.Error())
			}

			for _, repo := range tt.repos {
				repoFp, err := c.Put(repo)
				if err != nil {
					t.Fatalf("unexpected err putting to cache: %s", err.Error())
				}
				repoFp.Done()
			}

			validateCache(t, c, tt.expectedCacheOrdering)
		})
	}
}

func TestTryEvict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		cacheDir              string
		repos                 []string
		expectedCacheOrdering []string
		neverEvictRepos       map[string]bool
	}{
		{
			name:     "Evicts repos when size limit reached",
			cacheDir: t.TempDir(),
			repos: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/pizza-cli",
				"https://github.com/open-sauced/insights",
			},
			expectedCacheOrdering: []string{},
			neverEvictRepos:       map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewGitRepoLRUCache(tt.cacheDir, 1, tt.neverEvictRepos)
			if err != nil {
				t.Fatalf("unexpected err: %s", err.Error())
			}

			for _, repo := range tt.repos {
				repoFp, err := c.Put(repo)
				if err != nil {
					t.Fatalf("unexpected err putting to cache: %s", err.Error())
				}
				repoFp.lock.Unlock()
			}

			// Reset the cache with a very, very large min free Gb field
			// in order to force the eviction algorithm to evict all repos
			c.minFreeDiskGb = 10000000
			err = c.tryEvict()
			if err != nil {
				t.Fatalf("unexpected err attempting to evict repos: %s", err.Error())
			}

			validateCache(t, c, tt.expectedCacheOrdering)
		})
	}
}

func TestGetGitRepoLRUCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		cacheDir              string
		loadToCache           []string
		getFromCache          []string
		expectedCacheOrdering []string
		wantErr               bool
		wantNil               bool
		neverEvictRepos       map[string]bool
	}{
		{
			name:     "Gets queried repo and inserts it to front of cache",
			cacheDir: t.TempDir(),
			loadToCache: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/pizza-cli",
				"https://github.com/open-sauced/insights",
			},
			getFromCache: []string{
				"https://github.com/open-sauced/pizza",
			},
			expectedCacheOrdering: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/insights",
				"https://github.com/open-sauced/pizza-cli",
			},
			wantErr: false,
			wantNil: false,
			neverEvictRepos: map[string]bool{
				"no-test": true,
			},
		},
		{
			name:     "Returns nothing and no error if repo not in cache",
			cacheDir: t.TempDir(),
			loadToCache: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/pizza-cli",
				"https://github.com/open-sauced/insights",
			},
			getFromCache: []string{
				"https://github.com/open-sauced/ai",
			},
			expectedCacheOrdering: []string{
				"https://github.com/open-sauced/insights",
				"https://github.com/open-sauced/pizza-cli",
				"https://github.com/open-sauced/pizza",
			},
			wantErr: false,
			wantNil: true,
			neverEvictRepos: map[string]bool{
				"no-test": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewGitRepoLRUCache(tt.cacheDir, 1, tt.neverEvictRepos)
			if err != nil {
				t.Fatalf("unexpected err creating cache: %s", err.Error())
			}

			for _, repo := range tt.loadToCache {
				repoFp, err := c.Put(repo)
				if err != nil {
					t.Fatalf("unexpected err putting to cache: %s", err.Error())
				}
				repoFp.lock.Unlock()
			}

			for _, repo := range tt.getFromCache {
				repo := c.Get(repo)
				if err != nil && !tt.wantErr {
					t.Fatalf("unexpected err getting from cache: %s", err.Error())
				}

				if repo == nil && !tt.wantNil {
					t.Fatal("get returned a nil git repo")
				}

				if repo != nil {
					repo.Done()
				}
			}

			validateCache(t, c, tt.expectedCacheOrdering)
		})
	}
}

func TestGetAndPutConcurrently(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		cacheDir              string
		loadToCache           []string
		getFromCache          []string
		expectedCacheOrdering []string
		wantErr               bool
		wantNil               bool
		neverEvictRepos       map[string]bool
	}{
		{
			name:     "Gets queried repo and puts it to front of cache",
			cacheDir: t.TempDir(),
			loadToCache: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/pizza-cli",
				"https://github.com/open-sauced/insights",
			},
			getFromCache: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/pizza-cli",
				"https://github.com/open-sauced/insights",
			},
			expectedCacheOrdering: []string{
				"https://github.com/open-sauced/pizza",
				"https://github.com/open-sauced/insights",
				"https://github.com/open-sauced/pizza-cli",
			},
			wantErr: false,
			wantNil: false,
			neverEvictRepos: map[string]bool{
				"no-test": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewGitRepoLRUCache(tt.cacheDir, 1, tt.neverEvictRepos)
			if err != nil {
				t.Fatalf("unexpected err creating cache: %s", err.Error())
			}

			var wg sync.WaitGroup
			wg.Add(6)

			// Launch several go routines to in-parallel "Put" to the cache.
			for _, repo := range tt.loadToCache {
				go func(repo string, wg *sync.WaitGroup) {
					defer wg.Done()
					repoFp, _ := c.Put(repo)
					repoFp.lock.Unlock()
				}(repo, &wg)
			}

			// Launch several go routines to in-parallel "Get" from the cache
			for _, repo := range tt.getFromCache {
				go func(repo string, wg *sync.WaitGroup) {
					defer wg.Done()

					repoFp := c.Get(repo)
					if repoFp != nil {
						repoFp.Done()
					}
				}(repo, &wg)
			}

			// Wait for all threads to finish
			wg.Wait()

			// This is a soft validation.
			// Since putting and getting from the cache is performed concurrently,
			// there's no reliable way to know what the enevitable order of the cache
			// will be.
			if len(c.hm) != len(tt.expectedCacheOrdering) {
				t.Fatalf("cache hashmap not the expected size: %d, %d", len(c.hm), len(tt.expectedCacheOrdering))
			}

			if c.dll.Len() != len(tt.expectedCacheOrdering) {
				t.Fatalf("cache doubly linked list not the expected size: %d, %d", c.dll.Len(), len(tt.expectedCacheOrdering))
			}
		})
	}
}
