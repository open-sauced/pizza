// package insights provides data structures for insights powered by the pizza
// service. For now, only git commit insights are supported.
package insights

import "time"

// CommitInsight is the main internal data structure that represents a single
// git commit.
type CommitInsight struct {
	RepoURLSource string
	Hash          string
	AuthorEmail   string
	Date          time.Time
}
