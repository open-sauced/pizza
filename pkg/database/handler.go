// package database provides the pizza server with a wrapper around an
// sql database connection pool and the public methods to query and access
// that database
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	// the injected postgres interface implementations for Go SQL
	_ "github.com/lib/pq"

	"github.com/open-sauced/pizza/oven/pkg/insights"
)

// PizzaOvenDbHandler is a wrapper around *sql.DB. It provides a single
// point where internal methods and queries can access the Pizza oven database
// connection pool.
type PizzaOvenDbHandler struct {
	db *sql.DB
}

// NewPizzaOvenDbHandler builds a PizzaOvenDbHandler based on the provided
// database connection parameters
func NewPizzaOvenDbHandler(host, port, user, pwd, dbName string) *PizzaOvenDbHandler {
	connectString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=require", host, port, user, pwd, dbName)

	// Acquire the *sql.DB instance
	dbPool, err := sql.Open("postgres", connectString)
	if err != nil {
		log.Fatalf("Could not open database connection: %s", err)
	}

	// ping once to ensure the database values and connection are valid and working
	err = dbPool.Ping()
	if err != nil {
		log.Fatalf("Could not ping database: %s", err)
	}

	return &PizzaOvenDbHandler{
		db: dbPool,
	}
}

// GetRepositoryID queries the id of a repository based on its git URL
func (p PizzaOvenDbHandler) GetRepositoryID(insight insights.CommitInsight) (int, error) {
	var id int
	err := p.db.QueryRow("SELECT id FROM public.repos WHERE git_url=$1", insight.RepoURLSource).Scan(&id)
	return id, err
}

// InsertRepository inserts a git repository by its git_url
func (p PizzaOvenDbHandler) InsertRepository(insight insights.CommitInsight) (int, error) {
	var id int
	err := p.db.QueryRow("INSERT INTO public.repos(git_url) VALUES($1) RETURNING id", insight.RepoURLSource).Scan(&id)
	return id, err
}

// GetAuthorID queries the id of an author by their email
func (p PizzaOvenDbHandler) GetAuthorID(insight insights.CommitInsight) (int, error) {
	var id int
	err := p.db.QueryRow("SELECT id FROM public.users WHERE login=$1", insight.AuthorEmail).Scan(&id)
	return id, err
}

// InsertAuthor inserts an author by their email
func (p PizzaOvenDbHandler) InsertAuthor(insight insights.CommitInsight) (int, error) {
	var id int
	err := p.db.QueryRow("INSERT INTO public.users(login) VALUES($1) RETURNING id", insight.AuthorEmail).Scan(&id)
	return id, err
}

// GetCommitID queries the id of a given commit based on its hash
func (p PizzaOvenDbHandler) GetCommitID(repoID int, insight insights.CommitInsight) (int, error) {
	var id int
	err := p.db.QueryRow("SELECT id FROM public.commits WHERE repo_id=$1 AND commit_hash=$2", repoID, insight.Hash).Scan(&id)
	return id, err
}

// InsertCommit inserts a commit based on its commit hash
func (p PizzaOvenDbHandler) InsertCommit(insight insights.CommitInsight, authorID int, repoID int) error {
	_, err := p.db.Exec("INSERT INTO public.commits(commit_hash, user_id, repo_id, commit_date) VALUES($1, $2, $3, $4)", insight.Hash, authorID, repoID, insight.Date)
	return err
}

// GetLastCommit returns time.Time of the last git commit for the given repoID
func (p PizzaOvenDbHandler) GetLastCommit(repoID int) (time.Time, error) {
	var dateTime sql.NullTime
	err := p.db.QueryRow("SELECT commit_date FROM public.commits WHERE commit_date IS NOT NULL AND repo_id=$1 ORDER BY commit_date DESC LIMIT 1", repoID).Scan(&dateTime)
	if err != nil {
		if err == sql.ErrNoRows {
			// When no rows are returned, use an empty time.Time struct which
			// is "the beginning of time"
			return time.Time{}, nil
		}

		// If it's an error for anything else, return the error
		return time.Time{}, err
	}

	// If the returned date / time is not valid for some reason,
	// return an empty time struct
	if !dateTime.Valid {
		return time.Time{}, nil
	}

	return dateTime.Time, nil
}
