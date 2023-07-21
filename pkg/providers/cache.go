package providers

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"go.uber.org/zap"

	"github.com/open-sauced/pizza/oven/pkg/cache"
)

// NeverEvictRepos holds all the repos that must never be evicted in the LRU cache
// where the key is the URL of the repo
type NeverEvictRepos map[string]bool

// LRUCacheGitRepoProvider is a git repository provider that uses an internal
// Least Recently Used cache on disk for loading and querying git repositories.
// LRUCacheGitRepoProvider implements and statisfies the GitRepoProvider
// interface.
type LRUCacheGitRepoProvider struct {
	logger   *zap.SugaredLogger
	LRUCache *cache.GitRepoLRUCache
}

// NewLRUCacheGitRepoProvider returns a new LRUCacheGitRepoProvider using the
// configured cache directory and sets the minimum amount of free disk for the
// cache to keep.
func NewLRUCacheGitRepoProvider(cacheDir string, minFreeDisk uint64, l *zap.SugaredLogger, neverEvictRepos NeverEvictRepos) (GitRepoProvider, error) {
	cache, err := cache.NewGitRepoLRUCache(cacheDir, minFreeDisk, neverEvictRepos)
	if err != nil {
		return nil, fmt.Errorf("could not initialize a new LRU cache: %s", err.Error())
	}

	return &LRUCacheGitRepoProvider{
		logger:   l,
		LRUCache: cache,
	}, nil
}

// FetchRepo returns a CachedGitRepo which statisfies the GitRepo interface.
// It uses its internal LRU cache to "Get" and "Put". If a given git repo
// is not in the cache, FetchRepo will place it at the top of the cache where
// it will also be cloned to disk. See GitRepoLRUCache for details.
func (lc *LRUCacheGitRepoProvider) FetchRepo(URL string) (GitRepo, error) {
	var err error

	lc.logger.Debugf("Getting repo from LRU cache: %s", URL)

	repoInCache := lc.LRUCache.Get(URL)
	if repoInCache == nil {
		lc.logger.Debugf("Cache miss. Putting to cache: %s", URL)
		repoInCache, err = lc.LRUCache.Put(URL)
		if err != nil {
			return nil, fmt.Errorf("could not put to the git repo LRU cache: %s", err.Error())
		}
	}

	lc.logger.Debugf("Opening and fetching repo: %s", URL)
	repo, err := repoInCache.OpenAndFetch()
	if err != nil {
		return nil, fmt.Errorf("could not open and fetch repo: %s", err.Error())
	}

	return &CachedGitRepo{
		url:        URL,
		cacheEntry: repoInCache,
		repo:       repo,
	}, nil
}

// CachedGitRepo implements the GitRepo interface
type CachedGitRepo struct {
	url        string
	cacheEntry *cache.GitRepoFilePath
	repo       *git.Repository
}

// GetRepo returns the opened go-git repository
func (lc *CachedGitRepo) GetRepo() *git.Repository {
	return lc.repo
}

// Done closes the cached git repository making it available for other threads
// to start operating on this git repository.
//
// It is critical that "Done()" is called when operations are completed on a
// CachedGitRepo so the lock may be released for other threads to subsequently
// operate on that cached repo.
func (lc *CachedGitRepo) Done() {
	lc.cacheEntry.Done()
}
