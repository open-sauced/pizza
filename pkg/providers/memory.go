package providers

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"go.uber.org/zap"
)

// InMemoryGitRepoProvider implements and satisfies the GitRepoProvider
// interface
type InMemoryGitRepoProvider struct {
	Logger *zap.SugaredLogger
}

// NewInMemoryGitRepoProvider returns a new InMemoryGitRepoProvider using a
// configured logger
func NewInMemoryGitRepoProvider(logger *zap.SugaredLogger) GitRepoProvider {
	return &InMemoryGitRepoProvider{
		Logger: logger,
	}
}

// FetchRepo clones the configured repository into memory
func (im *InMemoryGitRepoProvider) FetchRepo(url string) (GitRepo, error) {
	inMemRepo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:          url,
		SingleBranch: true,
	})

	if err != nil {
		return nil, fmt.Errorf("could not clone in memory repo using in memory git repo provider: %s", err.Error())
	}

	return &InMemoryGitRepo{
		url:  url,
		repo: inMemRepo,
	}, nil
}

// InMemoryGitRepo satisfies and implements the GitRepo interface
type InMemoryGitRepo struct {
	url  string
	repo *git.Repository
}

// GetRepo returns the opened go-git repository
func (im *InMemoryGitRepo) GetRepo() *git.Repository {
	return im.repo
}

// Done is a no-opt for the in-memory git provider since there's nothing to do
func (im *InMemoryGitRepo) Done() {}
