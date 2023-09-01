package github

import (
	"context"
	"net/http"

	"github.com/google/go-github/v54/github"
)

type GithubClient struct {
	client *github.Client
}

func NewTokenClient(token string) *GithubClient {
	ctx := context.Background()
	s := &GithubClient{
		client: github.NewTokenClient(ctx, token),
	}
	return s
}

func NewClient(httpClient *http.Client) *GithubClient {
	s := &GithubClient{
		client: github.NewClient(httpClient),
	}
	return s
}

func (s *GithubClient) ListReposByOrg(org string) ([]*github.Repository, error) {
	ctx := context.Background()
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	// get all pages of results
	var allRepos []*github.Repository
	for {
		repos, resp, err := s.client.Repositories.ListByOrg(ctx, org, opt)
		if err != nil {
			return allRepos, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allRepos, nil
}

func FilterArchivedRepos(repos []*github.Repository) []*github.Repository {
	var filteredRepos []*github.Repository
	var falseVal bool = false
	for _, repo := range repos {
		if repo.Archived != &falseVal {
			filteredRepos = append(filteredRepos, repo)
		}
	}
	return filteredRepos
}

func GetRepoHTMLUrls(repos []*github.Repository) []string {
	var urls []string
	for _, repo := range repos {

		htmlUrl := repo.GetHTMLURL()
		if htmlUrl != "" {
			urls = append(urls, htmlUrl)
		}
	}
	return urls
}
