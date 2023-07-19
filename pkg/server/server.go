// package server serves the pizza service and provides the overall
// functionality.
package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/open-sauced/pizza/oven/pkg/database"
	"github.com/open-sauced/pizza/oven/pkg/insights"
	"go.uber.org/zap"
)

// PizzaOvenServer provides a leveled logger for use during serving requests
// and a PizzaOvenDbHanlder for accessing a sql pool of connections.
type PizzaOvenServer struct {
	Logger    *zap.SugaredLogger
	PizzaOven *database.PizzaOvenDbHandler
}

// envelope: type for json communications
type envelope map[string]any

// NewPizzaOvenServer returns a PizzaOvenServer with a new leveled logger
// which uses the provided PizzaOvenHandler for db connections
func NewPizzaOvenServer(dbHandler *database.PizzaOvenDbHandler) *PizzaOvenServer {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Could not initiate zap logger: %v", err)
	}
	sugarLogger := logger.Sugar()
	sugarLogger.Infof("initiated zap logger with level: %d", sugarLogger.Level())

	return &PizzaOvenServer{
		Logger:    sugarLogger,
		PizzaOven: dbHandler,
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
	URL string `json:"url"`
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

	err = p.processRepository(data.URL)
	if err != nil {
		p.Logger.Errorf("Could not process repository input: %v with error: %v", r.Body, err)
		http.Error(w, "Could not process input", http.StatusInternalServerError)
		return
	}
}

func (p PizzaOvenServer) pingHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("pong")); err != nil {
		p.Logger.Errorf("Could not connect to /ping endpoint: %v", err.Error())
		http.Error(w, "Could not connect, server is down", http.StatusInternalServerError)
	}
}

func (p PizzaOvenServer) processRepository(repo string) error {
	var err error

	insight := insights.CommitInsight{
		RepoURLSource: repo,
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

	p.Logger.Debugf("Cloning repo into memory: %s", insight.RepoURLSource)
	inMemRepo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL: repo,
	})
	if err != nil {
		return err
	}

	p.Logger.Debugf("Inspecting the head of the in memory repo: %s", insight.RepoURLSource)
	ref, err := inMemRepo.Head()
	if err != nil {
		return err
	}

	// Git shortlog options to display summary and email starting at HEAD
	gitLogOptions := git.LogOptions{
		From: ref.Hash(),
	}

	p.Logger.Debugf("Getting commit iterator with git log options: %v", gitLogOptions)
	commitIter, err := inMemRepo.Log(&gitLogOptions)
	if err != nil {
		return err
	}

	p.Logger.Debugf("Iterating commits in repository: %v", gitLogOptions)
	err = commitIter.ForEach(func(c *object.Commit) error {
		// TODO - if the committer and author are not the same, handle both
		// those users. This is the case where there is a separate committer for
		// a patch that was not authored by that specific person making the commit.

		// TODO - if there is a co-author, should handle adding that person on
		// the commit as well.
		insight.AuthorEmail = c.Author.Email
		insight.Hash = c.Hash.String()
		insight.Date = c.Committer.When

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

	return err
}

// errorResponse: utility to write errors in JSON and send it
func (p PizzaOvenServer) errorResponse(w http.ResponseWriter, status int, message any) {
	env := envelope{"error": message}

	err := p.WriteJSON(w, status, env, nil)
	if err != nil {
		p.Logger.Errorf("Could not write JSON, %v")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// failedValidationResponse: custom response to a failed validation
func (p PizzaOvenServer) failedValidationResponse(w http.ResponseWriter, errors map[string]string) {
	p.errorResponse(w, http.StatusUnprocessableEntity, errors)
}

// WriteJson: utility function to write json
func (p PizzaOvenServer) WriteJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')
	for key, val := range headers {
		w.Header()[key] = val
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}
