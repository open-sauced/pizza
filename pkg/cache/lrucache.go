package cache

import (
	"container/list"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/go-git/go-git/v5"

	"golang.org/x/sys/unix"
)

// GitRepoLRUCache is a Least Recently Used (LRU) "like" cache implemented with a
// doubly-linked-list and hashmap.
//
// It uses the GitRepoFilePath as elements and differs slightly from a typical LRU cache:
//   - Individual elements represent git cloned repos on-disk
//   - The GitRepoLRUCache evicts elements based on the configured minimum free disk in Gbs.
//     I.e., when the disk is filled to the point of surpassing the minFreeDiskGb
//     variable, the least recently used git repos on disk will be deleted from
//     the disk and evicted from the cache until free space on disk surpasses
//     the configured minFreeDiskGb.
//
// Further, it has the following additional properties:
//   - A locking mutex to support parallel processing of the cache itself
//   - Both "Get()" and "Put()" return the individual element in a locked state,
//     ready for processing. Callers should ALWAYS call "element.Done()" to unlock
//     the individual element once processing has completed.
type GitRepoLRUCache struct {
	// The locking mutex for operations on the cache itself (like updating the
	// position of elements in the cache or adding/deleting elements within the cache).
	// Not for use when processing individual elements returned from the cache.
	lock sync.Mutex

	// minFreeDiskGb is the minimum amount of available disk (in Gb) before the
	// cache will begin evicting elements.
	minFreeDiskGb uint64

	// dir is the directory to store clone repos on-disk
	dir string

	// dll is the doubly linked list to support the LRU cache behavior
	dll *list.List

	// hm is the hashmap to support the LRU cache behavior
	hm map[string]*list.Element

	// neverEvictRepos are the repositories that must never be evicted from the LRU cache
	neverEvictRepos map[string]bool
}

// NewGitRepoLRUCache returns a new NewGitRepoLRUCache configured with the
// destination directory to cache git repos and minimum free gbs
func NewGitRepoLRUCache(dir string, minFreeGbs uint64, neverEvictRepos map[string]bool) (*GitRepoLRUCache, error) {
	path := filepath.Clean(dir)
	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error checking provided cache directory: %s", err.Error())
	}

	stats := &syscall.Statfs_t{}

	err = syscall.Statfs(dir, stats)
	if err != nil {
		return nil, fmt.Errorf("error fetching stats for cache directory: %s", err.Error())
	}

	freeSpace := stats.Bavail * uint64(stats.Bsize)
	minFreeBytes := minFreeGbs * 1024 * 1024 * 1024

	if freeSpace <= minFreeBytes {
		return nil, fmt.Errorf("minimum free disk space: %d exceeds actual available disk space: %d", minFreeBytes, freeSpace)
	}

	return &GitRepoLRUCache{
		minFreeDiskGb:   minFreeGbs,
		dir:             path,
		dll:             list.New(),
		hm:              make(map[string]*list.Element),
		neverEvictRepos: neverEvictRepos,
	}, nil
}

// Get checks the GitRepoLRUCache for the provided key and returns the associated
// GitRepoFilePath element if present, bumping it to the front of the cache.
// If not present, returns nil.
func (c *GitRepoLRUCache) Get(key string) *GitRepoFilePath {
	c.lock.Lock()
	defer c.lock.Unlock()

	if element, ok := c.hm[key]; ok {
		// Cache hit
		c.dll.MoveToFront(element)
		element.Value.(*GitRepoFilePath).lock.Lock()
		return element.Value.(*GitRepoFilePath)
	}

	// Cache miss
	return nil
}

// Put clones a git repo to disk and adds it to the GitRepoLRUCache. If the
// element is already in the cache, it simply moves that element to the front
// of the cache. Put will also attempt to call "tryEvict" when adding new repos
// to ensure the cache has not surpassed the minimum amount of free disk.
//
// Unlocking the cache is done manually (and not through "defer c.lock.Unlock()"
// in order to free other threads to perform cache operations when possibly
// lengthy git cloning operations are being performed on individual elements.
func (c *GitRepoLRUCache) Put(key string) (*GitRepoFilePath, error) {
	c.lock.Lock()

	if element, ok := c.hm[key]; ok {
		// Cache hit, early return
		c.dll.MoveToFront(element)
		element.Value.(*GitRepoFilePath).lock.Lock()
		c.lock.Unlock()
		return element.Value.(*GitRepoFilePath), nil
	}

	// Cache miss, create new element and clone to disk

	// Calculate free disk space and evict repos as needed before cloning new ones
	err := c.tryEvict()
	if err != nil {
		c.lock.Unlock()
		return nil, fmt.Errorf("could not evict repos from cache: %s", err.Error())
	}

	pathKey := filepath.Join(c.dir, key)

	// Create a new element in the cache
	element := &GitRepoFilePath{
		key:  key,
		path: pathKey,
	}

	c.hm[key] = c.dll.PushFront(element)

	// Lock the newly created element before unlocking the cache
	element.lock.Lock()

	// At this point, now that the cache itself has been updated with the new
	// element, we can unlock the cache to allow for operations on other elements
	// in the cache. Since cloning (given network conditions, size of repository,
	// etc.) may take abit of time, unlocking the cache at this stage before cloning
	// of the new repository begins releases a bottleneck for other repos to be processed.
	// Because the newly created element is locked, it is safe to continue
	// cache operations before the new repo has been cloned on-disk (without risk
	// of it being evicted)
	c.lock.Unlock()

	// Check the directory based on the input key
	_, err = os.Stat(pathKey)
	if err == nil {
		// If the "os.Stat(...)" call was successful, this means the directory
		// exists already on disk. It's possible (after a container restart, if
		// a new disk has been attached, etc.) that there are existing elements
		// on-disk that correspond to valid git repos.
		//
		// This branch validates that the directory is a valid git repo, can be used,
		// and continues without having to re-clone it.
		_, err = git.PlainOpen(pathKey)
		if err == nil {
			// At this point, if the repo can be "git-opened" on disk, it's a
			// valid repo and can be used. So, return the existing element that
			// points to this path.
			return element, nil
		}

		// Otherwise, the repo is somehow invalid and should be removed from disk.
		os.RemoveAll(pathKey)
	}

	// Create the directory and all its parent dirs
	err = os.MkdirAll(pathKey, os.ModePerm)
	if err != nil {
		element.lock.Unlock()
		return nil, fmt.Errorf("could not create directory in cache: %s", err.Error())
	}

	// Clone the new repo to disk
	_, err = git.PlainClone(pathKey, false, &git.CloneOptions{
		URL:  key,
		Tags: git.NoTags,
	})
	if err != nil {
		element.lock.Unlock()
		return nil, fmt.Errorf("could not clone into cache directory: %s", err.Error())
	}

	// Return the GitRepoFilePath element (which is still locked to allow for
	// additional processing)
	return element, nil
}

// tryEvict calculates the available bytes, compares that to the cache's
// minFreeDiskGb field and evicts the least recently used elements until
// there is enough free disk space.
func (c *GitRepoLRUCache) tryEvict() error {
	var stat unix.Statfs_t
	err := unix.Statfs(c.dir, &stat)
	if err != nil {
		return fmt.Errorf("could not calculate disk space using statfs: %s", err.Error())
	}

	// lazy convert gb -> mb -> kb -> bytes
	minFreeBytes := c.minFreeDiskGb * 1024 * 1024 * 1024

	// Available bytes within cache directory * size of byte blocks on the system
	// compared to the minimum amount of free disk in Gb converted on the fly
	// to bytes
	for stat.Bavail*uint64(stat.Bsize) <= minFreeBytes {
		// Early exit if the cache is empty
		if c.dll.Back() == nil {
			break
		}

		// Check that the LRU element is not part of neverEvictRepos
		// If it is, find the nearest LRU not part of neverEvictRepos
		lruNode := c.dll.Back()
		for _, isInNeverEvictRepos := c.neverEvictRepos[lruNode.Value.(*GitRepoFilePath).key]; isInNeverEvictRepos; {
			lruNode = lruNode.Next()
			if lruNode == nil {
				return fmt.Errorf("Disk space completely occupied by neverEvictRepos, could not evict")
			}
		}

		// Attempt to unlock the individual element
		lruNode.Value.(*GitRepoFilePath).lock.Lock()

		// Evict least recently used repos
		os.RemoveAll(lruNode.Value.(*GitRepoFilePath).path)
		delete(c.hm, lruNode.Value.(*GitRepoFilePath).key)
		c.dll.Remove(lruNode)

		// Recalculate the free bytes
		err = unix.Statfs(c.dir, &stat)
		if err != nil {
			return fmt.Errorf("could not re-calculate disk space using statfs: %s", err.Error())
		}
	}

	return nil
}
