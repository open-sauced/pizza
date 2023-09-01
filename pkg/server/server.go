// package server serves the pizza service and provides the overall
// functionality.
package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/open-sauced/pizza/oven/pkg/common"
	"github.com/open-sauced/pizza/oven/pkg/database"
	"github.com/open-sauced/pizza/oven/pkg/github"
	"github.com/open-sauced/pizza/oven/pkg/insights"
	"github.com/open-sauced/pizza/oven/pkg/providers"
)

// counter is a atomic counter that is used to create canonical, short lived
// temporary table names for bulk inserts of commit authors
var counter int64

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

func (p PizzaOvenServer) processOrg(orgUrlString string, processArchived bool) ([]string, error) {

	var htmlUrls []string
	orgUrl, err := url.Parse(orgUrlString)
	if err != nil {
		return htmlUrls, err
	}
	if orgUrl.Hostname() == "github.com" {
		client := github.NewClient(nil)
		orgName := strings.Trim(orgUrl.Path, "/")
		repos, err := client.ListReposByOrg(orgName)
		if err != nil {
			return htmlUrls, fmt.Errorf("Error encountered fetching repositories, org: %s with error: %s ", orgName, err)
		}
		if !processArchived {
			repos = github.FilterArchivedRepos(repos)
		}
		htmlUrls = github.GetRepoHTMLUrls(repos)

		return htmlUrls, nil
	} else {
		return htmlUrls, fmt.Errorf("Cannot parse organizations from %s", orgUrl.Hostname())
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

	p.Logger.Debugf("Validating and normalizing repository URL: %s", data.URL)
	normalizedRepoURL, err := common.NormalizeGitURL(data.URL)
	if err != nil {
		p.Logger.Debugf("Could not normalize repo URL %s: %s", data.URL, err.Error())
		http.Error(w, fmt.Sprintf("Could not normalize provided repo URL: %s", err.Error()), http.StatusBadRequest)
		return
	}

	repoURLendpoint, err := transport.NewEndpoint(normalizedRepoURL)
	if err != nil {
		p.Logger.Errorf("Could not create git transport endpoint with repo URL %s: %s", data.URL, err.Error())
		http.Error(w, fmt.Sprintf("Could not create git transport endpoint from provided repo URL: %s", err.Error()), http.StatusBadRequest)
		return
	}

	ok, err := common.IsValidGitRepo(repoURLendpoint.String())
	if !ok {
		if err != nil {
			p.Logger.Errorf("Error validating repo URL %s: %s", data.URL, err.Error())
			http.Error(w, fmt.Sprintf("Error validating remote git repo URL: %s", err.Error()), http.StatusBadRequest)
			return
		}

		p.Logger.Debug("Could not validate repo URL %s: %s", data.URL, err.Error())
		http.Error(w, fmt.Sprintf("not valid git repo URL. Expected format protocol://address but got: %s", err.Error()), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	if data.Wait {
		if data.Org != "" {
			cloneUrls, err := p.processOrg(data.Org, data.Archives)
			if err != nil {
				p.Logger.Errorf("Could not process org input: %v with error: %v", r.Body, err)
				http.Error(w, "Could not process input", http.StatusInternalServerError)
				return
			}
			for _, cloneUrl := range cloneUrls {
				err = p.processRepository(cloneUrl)
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
			cloneUrls, err := p.processOrg(data.Org, data.Archives)
			if err != nil {
				p.Logger.Errorf("Could not process org input: %v with error: %v", r.Body, err)
				http.Error(w, "Could not process input", http.StatusInternalServerError)
				return
			}
			errors := make([]string, 0)
			for _, cloneUrl := range cloneUrls {
				go func(cloneUrl string) {
					err = p.processRepository(cloneUrl)
					if err != nil {
						errors = append(errors, fmt.Sprintf("Could not process repo clone URL: %v with error: %v", cloneUrl, err))
					}

				}(cloneUrl)
			}
			if len(errors) > 0 {
				errorString := strings.Join(errors, "\n")
				p.Logger.Error(errorString)
				http.Error(w, "Could not process input", http.StatusInternalServerError)
			}
			return
		}
		go func() {
			err = p.processRepository(repoURLendpoint.String())
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
	providedRepo, err := p.PizzaGitProvider.FetchRepo(insight.RepoURLSource)
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

	// Add 1 nanosecond since "git log --since" is inclusive of date/times.
	// Although date/times are not unique to commits, it is incredibly unlikely that
	// two commits will have the exact same timestamp and be excluded using this method
	latestCommitDate = latestCommitDate.Add(time.Nanosecond)
	p.Logger.Debugf("Querying commits since: %s", latestCommitDate.String())

	// Git shortlog options to display summary and email starting at HEAD
	gitLogOptions := git.LogOptions{
		From:  ref.Hash(),
		Since: &latestCommitDate,
	}

	p.Logger.Debugf("Getting commit iterator with git log options: %v", gitLogOptions)
	authorIter, err := gitRepo.Log(&gitLogOptions)
	if err != nil {
		return err
	}

	// Build a unique, atomically safe temporary table name to pivot commit
	// author data from
	rawUUID := uuid.New().String()
	uuid := strings.ReplaceAll(rawUUID, "-", "")
	tmpTableName := fmt.Sprintf("temp_table_%s_%d", uuid, atomic.AddInt64(&counter, 1))

	p.Logger.Debugf("Using temporary db table for commit authors: %s", tmpTableName)
	authorTxn, authorStmt, err := p.PizzaOven.PrepareBulkAuthorInsert(tmpTableName)
	if err != nil {
		return err
	}

	// To reduce unnecessary duplicate statement executions, track the unique
	// author emails using a simple set (represented as a string map to structs)
	uniqueAuthorEmails := []string{}
	authorEmailSet := make(map[string]struct{})

	p.Logger.Debugf("Iterating commit authors in repository: %s with temporary tablename: %s", insight.RepoURLSource, tmpTableName)
	err = authorIter.ForEach(func(c *object.Commit) error {
		// TODO - if the committer and author are not the same, handle both
		// those users. This is the case where there is a separate committer for
		// a patch that was not authored by that specific person making the commit.

		// TODO - if there is a co-author, should handle adding that person on
		// the commit as well.

		// Check if the author email is in the unique set of author emails
		if _, ok := authorEmailSet[c.Author.Email]; ok {
			return nil
		}

		// Commit author is not in set so add this author's email as unique
		authorEmailSet[c.Author.Email] = struct{}{}
		uniqueAuthorEmails = append(uniqueAuthorEmails, c.Author.Email)

		p.Logger.Debugf("Inspecting commit author: %s", c.Author.Email)
		return p.PizzaOven.InsertAuthor(authorStmt, insights.CommitInsight{
			RepoURLSource: repoURL,
			AuthorEmail:   c.Author.Email,
			Hash:          "",
			Date:          time.Time{},
		})
	})
	if err != nil {
		return err
	}

	// Resolve, execute, and pivot the bulk author transaction
	err = p.PizzaOven.ResolveTransaction(authorTxn, authorStmt)
	if err != nil {
		return err
	}

	err = p.PizzaOven.PivotTmpTableToAuthorsTable(tmpTableName)
	if err != nil {
		return err
	}

	// Re-query the database for author email ids based on the unique list of
	// author emails that have just been committed
	authorEmailIDMap, err := p.PizzaOven.GetAuthorIDs(uniqueAuthorEmails)
	if err != nil {
		return err
	}

	// Rebuild the iterator from the start using the same options
	commitIter, err := gitRepo.Log(&gitLogOptions)
	if err != nil {
		return err
	}

	// Get ready for the commit bulk action
	commitTxn, commitStmt, err := p.PizzaOven.PrepareBulkCommitInsert()
	if err != nil {
		return err
	}

	p.Logger.Debugf("Iterating commits in repository: %s", insight.RepoURLSource)
	err = commitIter.ForEach(func(c *object.Commit) error {
		i := insights.CommitInsight{
			RepoURLSource: repoURL,
			AuthorEmail:   c.Author.Email,
			Hash:          c.Hash.String(),
			Date:          c.Committer.When.UTC(),
		}

		p.Logger.Debugf("Inspecting commit: %s %s %s", i.AuthorEmail, i.Hash, i.Date)
		err = p.PizzaOven.InsertCommit(commitStmt, i, authorEmailIDMap[i.AuthorEmail], repoID)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Execute and resolve the bulk commit insert
	err = p.PizzaOven.ResolveTransaction(commitTxn, commitStmt)
	if err != nil {
		return err
	}

	p.Logger.Debugf("Finished processing: %s", insight.RepoURLSource)
	return nil
}
