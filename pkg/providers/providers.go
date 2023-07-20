package providers

import "github.com/go-git/go-git/v5"

// GitRepoProvider is an API for accessing git repositories.
// Different implementers of GitRepoProvider may
type GitRepoProvider interface {
	// FetchRepo is a single interface to acquire a GitRepo based on a provided
	// URL. Different
	FetchRepo(URL string) (GitRepo, error)
}

// GitRepo wraps individual git repositories with the necessary internal methods
// and structs provided by an GitRepoProvider. I.e., it allows for various
// GitRepoProviders to offer a flat API surface where individual git repos
// of different implementation may be accessed.
type GitRepo interface {
	// GetRepo returns the internal go-git git repository.
	GetRepo() *git.Repository

	// Done indicates that there is no more processing to be performed on the
	// GitRepo and any resources internal to the individual GitRepo may be
	// reaped and cleaned up.
	Done()
}
