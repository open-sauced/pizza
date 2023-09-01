// package server serves the pizza service and provides the overall
// functionality.
package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"go.uber.org/zap"

	"github.com/open-sauced/pizza/oven/pkg/database"
	"github.com/open-sauced/pizza/oven/pkg/insights"
	"github.com/open-sauced/pizza/oven/pkg/providers"
)

// Config provides the configuration set on server startup
// - Never Evict Repos: Repos that are preserved in cache regardless of LRU policy
type Config struct {
	NeverEvictRepos providers.NeverEvictRepos
}

// PizzaOvenServer provides a leveled logger for use during serving requests
// and a PizzaOvenDbHanlder for accessing a sql pool of connections.
type PizzaOvenServer struct {
	Logger           *zap.SugaredLogger
	PizzaOven        *database.PizzaOvenDbHandler
	PizzaGitProvider providers.GitRepoProvider
}

// NewPizzaOvenServer returns a PizzaOvenServer with a new leveled logger
// which uses the provided PizzaOvenHandler for db connections
func NewPizzaOvenServer(dbHandler *database.PizzaOvenDbHandler, provider providers.GitRepoProvider, sugarLogger *zap.SugaredLogger) *PizzaOvenServer {
	return &PizzaOvenServer{
		Logger:           sugarLogger,
		PizzaOven:        dbHandler,
		PizzaGitProvider: provider,
	}
}

// Run starts the http server on the provided port
func (p PizzaOvenServer) Run(serverPort string) {
	//nolint:errcheck
	defer p.Logger.Sync()
	p.Logger.Infof("Starting server on port %s", serverPort)
	http.HandleFunc("/bake", p.handleRequest)
	http.HandleFunc("/ping", p.pingHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", serverPort), nil))
}

type reqData struct {
	URL      string `json:"url,omitempty"`
	Wait     bool   `json:"wait,omitempty"`
	Org      string `json:"org,omitempty"`
	Archives bool   `json:"archives,omitempty"`
}

type orgRepo struct {
	URL      string `json:"html_url"`
	Archived bool   `json:"archived"`
}
type orgRepoList []orgRepo

func (p PizzaOvenServer) processOrg(orgUrlString string, processArchived bool) (orgRepoList, error) {
	repoList := make(orgRepoList, 0)
	orgUrl, err := url.Parse(orgUrlString)
	if err != nil {
		return repoList, err
	}
	if orgUrl.Hostname() == "github.com" {
		orgApiUrlString := fmt.Sprintf("https://api.github.com/orgs%s/repos", orgUrl.Path)
		res, err := http.Get(orgApiUrlString)
		if err != nil {
			return repoList, fmt.Errorf("Unable to request repo list from %s", orgApiUrlString)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		json.Unmarshal(body, &repoList)
		if processArchived {
			return repoList, nil
		}
		filteredRepoList := make(orgRepoList, 0)
		for _, repo := range repoList {
			if !repo.Archived {
				filteredRepoList = append(filteredRepoList, repo)
			}
		}
		return filteredRepoList, nil
	} else {
		return repoList, fmt.Errorf("Cannot parse organizations from %s", orgUrl.Hostname())
	}
}

func (p PizzaOvenServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.Logger.Errorf("Received request with invalid method: %v", r.Body)
		http.Error(w, "Invalid request method, expected post", http.StatusMethodNotAllowed)
		return
	}

	var data reqData
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		p.Logger.Errorf("Could not decode request json body: %v with error: %v", r.Body, err)
		http.Error(w, "Could not decode request body", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	if data.Wait {
		if data.Org != "" {
			repoList, err := p.processOrg(data.Org, data.Archives)
			if err != nil {
				p.Logger.Errorf("Could not process org input: %v with error: %v", r.Body, err)
				http.Error(w, "Could not process input", http.StatusInternalServerError)
				return
			}
			for _, repo := range repoList {
				err = p.processRepository(repo.URL)
			}
			return
		}
		err = p.processRepository(data.URL)
		if err != nil {
			p.Logger.Errorf("Could not process repository input: %v with error: %v", r.Body, err)
			http.Error(w, "Could not process input", http.StatusInternalServerError)
			return
		}
	} else {
		if data.Org != "" {
			repoList, err := p.processOrg(data.Org, data.Archives)
			if err != nil {
				p.Logger.Errorf("Could not process org input: %v with error: %v", r.Body, err)
				http.Error(w, "Could not process input", http.StatusInternalServerError)
				return
			}
			errors := make([]string, 0)
			for _, repo := range repoList {
				go func(repo orgRepo) {
					err = p.processRepository(repo.URL)
					if err != nil {
						errors = append(errors, fmt.Sprintf("Could not process repo: %v with error: %v", repo, err))
					}

				}(repo)
			}
			if len(errors) > 0 {
				errorString := strings.Join(errors, "\n")
				p.Logger.Error(errorString)
				http.Error(w, "Could not process input", http.StatusInternalServerError)
			}
			return
		}
		go func() {
			err = p.processRepository(data.URL)
			if err != nil {
				p.Logger.Errorf("Could not process repository input: %v with error: %v", r.Body, err)
				http.Error(w, "Could not process input", http.StatusInternalServerError)
				return
			}
		}()
	}
}

func (p PizzaOvenServer) pingHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("pong")); err != nil {
		p.Logger.Errorf("Could not connect to /ping endpoint: %v", err.Error())
		http.Error(w, "Could not connect, server is down", http.StatusInternalServerError)
	}
}

func (p PizzaOvenServer) processRepository(repoURL string) error {
	var err error

	insight := insights.CommitInsight{
		RepoURLSource: repoURL,
		AuthorEmail:   "",
		Hash:          "",
		Date:          time.Time{},
	}

	p.Logger.Debugf("Checking if repository is already in database: %s", insight.RepoURLSource)
	repoID, err := p.PizzaOven.GetRepositoryID(insight)
	if err != nil {
		if err == sql.ErrNoRows {
			p.Logger.Debugf("No repo found in db. Inserting repo: %s", insight.RepoURLSource)
			repoID, err = p.PizzaOven.InsertRepository(insight)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	p.Logger.Debugf("Getting repo via configured git provider: %s", insight.RepoURLSource)

	// Use the configured git provider to get the repo
	providedRepo, err := p.PizzaGitProvider.FetchRepo(repoURL)
	if err != nil {
		return err
	}
	defer providedRepo.Done()

	gitRepo := providedRepo.GetRepo()

	p.Logger.Debugf("Inspecting the head of the git repo: %s", insight.RepoURLSource)
	ref, err := gitRepo.Head()
	if err != nil {
		return err
	}

	p.Logger.Debugf("Getting last commit in DB: %s", insight.RepoURLSource)
	latestCommitDate, err := p.PizzaOven.GetLastCommit(repoID)
	if err != nil {
		return err
	}

	p.Logger.Debugf("Querying commits since: %s", latestCommitDate.String())

	// Git shortlog options to display summary and email starting at HEAD
	gitLogOptions := git.LogOptions{
		From:  ref.Hash(),
		Since: &latestCommitDate,
	}

	p.Logger.Debugf("Getting commit iterator with git log options: %v", gitLogOptions)
	commitIter, err := gitRepo.Log(&gitLogOptions)
	if err != nil {
		return err
	}

	p.Logger.Debugf("Iterating commits in repository: %s", repoURL)
	err = commitIter.ForEach(func(c *object.Commit) error {
		// TODO - if the committer and author are not the same, handle both
		// those users. This is the case where there is a separate committer for
		// a patch that was not authored by that specific person making the commit.

		// TODO - if there is a co-author, should handle adding that person on
		// the commit as well.
		insight.AuthorEmail = c.Author.Email
		insight.Hash = c.Hash.String()
		insight.Date = c.Committer.When.UTC()

		p.Logger.Debugf("Inspecting commit: %s %s %s", insight.AuthorEmail, insight.Hash, insight.Date)
		authorID, err := p.PizzaOven.GetAuthorID(insight)
		if err != nil {
			if err == sql.ErrNoRows {
				p.Logger.Debugf("Author not found. Inserting: %s %s %s", insight.AuthorEmail, insight.Hash, insight.Date)
				authorID, err = p.PizzaOven.InsertAuthor(insight)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		p.Logger.Debugf("Checking if commit already in database: %s", insight.Hash)
		_, err = p.PizzaOven.GetCommitID(repoID, insight)
		if err != nil {
			if err == sql.ErrNoRows {
				p.Logger.Debugf("Commit not found. Inserting into database: %s", insight.Hash)
				err = p.PizzaOven.InsertCommit(insight, authorID, repoID)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		return nil
	})

	p.Logger.Debugf("Finished processing: %s", repoURL)
	return err
}
