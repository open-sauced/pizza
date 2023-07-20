package cache

import (
	"sync"

	"github.com/go-git/go-git/v5"
)

// GitRepoFilePath is a key / value pair with a locking mutex which represents
// the key to a git repository (typically the remote URL) and its file path on disk.
// This is used as the primary element in GitRepoLRUCache.
//
// When processing and operations are completed for an individual GitRepoFilePath,
// always call "Done" to ensure no deadlocks occur on individual elements within
// a given GItRepoLRUCache.
// Example: "repo.Done()"
type GitRepoFilePath struct {
	// A locking mutex is used to ensure that on-disk git repos are not
	// modified during processing.
	// Locking is done manually via "element.lock.Lock()" within the cache package.
	// Once operations are completed, in order to free up the resource, the "Done()"
	// method should be called.
	lock sync.Mutex

	// The key for the GitRepoFilePath key/value pair, generally, is the
	// remote URL for the git repository
	key string

	// path is the value in the GitRepoFilePath key/value and denotes the
	// filepath on-disk to the cloned git repository
	path string
}

// OpenAndFetch opens a git repository on-disk and fetches the latest changes.
// If the git.NoErrAlreadyUpToDate error is produced, this function does not
// return an error but, instead, continues and returns the repo.
func (g *GitRepoFilePath) OpenAndFetch() (*git.Repository, error) {
	repo, err := git.PlainOpen(g.path)
	if err != nil {
		return nil, err
	}

	// Get the worktree for the repository
	w, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	// Pull the latest changes from the origin remote and merge into the current branch
	err = w.Pull(&git.PullOptions{})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, err
	}

	return repo, nil
}

// Done is a thin wrapper for unlocking the GitRepoFilePath's mutex.
// This should ALWAYS be called when operations and processing for this
// individual on-disk repo are completed in order to prevent a deadlock.
func (g *GitRepoFilePath) Done() {
	g.lock.Unlock()
}
