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
	"github.com/lib/pq"

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
	err := p.db.QueryRow("SELECT id FROM public.baked_repos WHERE clone_url=$1", insight.RepoURLSource).Scan(&id)
	return id, err
}

// InsertRepository inserts a git repository by its git_url
func (p PizzaOvenDbHandler) InsertRepository(insight insights.CommitInsight) (int, error) {
	var id int
	err := p.db.QueryRow("INSERT INTO public.baked_repos(clone_url) VALUES($1) RETURNING id", insight.RepoURLSource).Scan(&id)
	return id, err
}

// GetAuthorID queries the id of an author by their email
func (p PizzaOvenDbHandler) GetAuthorID(insight insights.CommitInsight) (int, error) {
	var id int
	err := p.db.QueryRow("SELECT id FROM public.commit_authors WHERE commit_author_email=$1", insight.AuthorEmail).Scan(&id)
	return id, err
}

// GetAuthorIDs queries the id of an author by their email
func (p PizzaOvenDbHandler) GetAuthorIDs(emails []string) (map[string]int, error) {
	emailIDMap := make(map[string]int)

	rows, err := p.db.Query("SELECT id, commit_author_email FROM commit_authors WHERE commit_author_email = ANY($1);", pq.Array(emails))
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var email string
		if err := rows.Scan(&id, &email); err != nil {
			log.Fatal(err)
		}
		emailIDMap[email] = id
	}

	return emailIDMap, nil
}

// PrepareBulkAuthorInsert creates a temporary table that mirrors the commit_authors
// and is used to perform a bulk insert "pivot" which accounts for conflicts
func (p PizzaOvenDbHandler) PrepareBulkAuthorInsert(tmpTableName string) (*sql.Tx, *sql.Stmt, error) {
	_, err := p.db.Exec(fmt.Sprintf("CREATE TEMPORARY TABLE %s AS SELECT * FROM commit_authors WHERE 1=0", tmpTableName))
	if err != nil {
		return nil, nil, err
	}

	txn, err := p.db.Begin()
	if err != nil {
		return nil, nil, err
	}

	stmt, err := txn.Prepare(pq.CopyIn(tmpTableName, "commit_author_email"))
	if err != nil {
		newErr := txn.Rollback()
		if newErr != nil {
			return nil, nil, fmt.Errorf("could not abort the sql transaction: %s - original error: %s", newErr, err)
		}

		return nil, nil, err
	}

	return txn, stmt, nil
}

// PivotTmpTableToAuthorsTable performs the pivot from the temporary commit authors
// table to the real one handling any conflicts
func (p PizzaOvenDbHandler) PivotTmpTableToAuthorsTable(tmpTableName string) error {
	_, err := p.db.Exec(fmt.Sprintf(`
		INSERT INTO public.commit_authors(commit_author_email)
		SELECT commit_author_email FROM %s
		ON CONFLICT (commit_author_email)
		DO NOTHING
	`, tmpTableName))
	if err != nil {
		return err
	}

	_, err = p.db.Exec(fmt.Sprintf("DROP TABLE %s", tmpTableName))
	if err != nil {
		return err
	}

	return nil
}

// InsertAuthor inserts an author by their email into the sql transaction
func (p PizzaOvenDbHandler) InsertAuthor(stmt *sql.Stmt, insight insights.CommitInsight) error {
	_, err := stmt.Exec(insight.AuthorEmail)
	return err
}

// PrepareBulkCommitInsert gets a sql bulk transaction ready to insert all commits
// from processing in one round trip
func (p PizzaOvenDbHandler) PrepareBulkCommitInsert() (*sql.Tx, *sql.Stmt, error) {
	txn, err := p.db.Begin()
	if err != nil {
		return nil, nil, err
	}

	stmt, err := txn.Prepare(pq.CopyIn("commits", "commit_hash", "commit_author_id", "baked_repo_id", "commit_date"))
	if err != nil {
		newErr := txn.Rollback()
		if newErr != nil {
			return nil, nil, fmt.Errorf("could not abort commits bulk sql transaction: %s - original error: %s", newErr, err)
		}

		return nil, nil, err
	}

	return txn, stmt, nil
}

// ResolveTransaction resolves a given transaction and sql statement
func (p PizzaOvenDbHandler) ResolveTransaction(txn *sql.Tx, stmt *sql.Stmt) error {
	_, err := stmt.Exec()
	if err != nil {
		return err
	}

	err = stmt.Close()
	if err != nil {
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}

// InsertCommit adds a commit to the given sql.Stmt to be executed in bulk
func (p PizzaOvenDbHandler) InsertCommit(stmt *sql.Stmt, insight insights.CommitInsight, authorID int, repoID int) error {
	_, err := stmt.Exec(insight.Hash, authorID, repoID, insight.Date)
	return err
}

// GetLastCommit returns time.Time of the last git commit for the given repoID
func (p PizzaOvenDbHandler) GetLastCommit(repoID int) (time.Time, error) {
	var dateTime sql.NullTime
	err := p.db.QueryRow("SELECT commit_date FROM public.commits WHERE commit_date IS NOT NULL AND baked_repo_id=$1 ORDER BY commit_date DESC LIMIT 1", repoID).Scan(&dateTime)
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
