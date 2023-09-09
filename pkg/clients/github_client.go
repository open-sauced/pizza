package clients

import (
	"context"
	"net/http"

	"github.com/google/go-github/v54/github"
)

type GithubApiClient struct {
	client *github.Client
}

func NewGithubTokenClient(token string) *GithubApiClient {
	ctx := context.Background()
	s := &GithubApiClient{
		client: github.NewTokenClient(ctx, token),
	}
	return s
}

func NewGithubClient(httpClient *http.Client) *GithubApiClient {
	s := &GithubApiClient{
		client: github.NewClient(httpClient),
	}
	return s
}

func (s *GithubApiClient) ListReposByOrg(org string) ([]*github.Repository, error) {
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

func FilterGithubArchivedRepos(repos []*github.Repository) []*github.Repository {
	var filteredRepos []*github.Repository
	for _, repo := range repos {
		if !*repo.Archived {
			filteredRepos = append(filteredRepos, repo)
		}
	}
	return filteredRepos
}

func GetGithubRepoHTMLUrls(repos []*github.Repository) []string {
	var urls []string
	for _, repo := range repos {

		htmlURL := repo.GetHTMLURL()
		if htmlURL != "" {
			urls = append(urls, htmlURL)
		}
	}
	return urls
}
