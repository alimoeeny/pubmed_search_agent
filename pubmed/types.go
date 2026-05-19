// Package pubmed provides types and an HTTP client for the NCBI PubMed E-utilities API.
package pubmed

// SortOrder controls the esearch result ordering.
type SortOrder string

const (
	SortRelevance   SortOrder = "relevance"    // esearch &sort=relevance
	SortMostRecent  SortOrder = "most_recent"  // esearch &sort=pub_date
	SortFirstAuthor SortOrder = "first_author" // esearch &sort=first_author
	SortJournal     SortOrder = "journal"      // esearch &sort=journal
)

// StudyType constrains which publication types are included in a query.
type StudyType string

const (
	StudyTypeRCT           StudyType = "randomized_controlled_trial"
	StudyTypeMetaAnalysis  StudyType = "meta_analysis"
	StudyTypeReview        StudyType = "review"
	StudyTypeObservational StudyType = "observational"
	StudyTypeClinicalTrial StudyType = "clinical_trial"
)

// pubTypeFilter maps a StudyType to its PubMed publication-type filter term.
var pubTypeFilter = map[StudyType]string{
	StudyTypeRCT:           "Randomized Controlled Trial[pt]",
	StudyTypeMetaAnalysis:  "Meta-Analysis[pt]",
	StudyTypeReview:        "Review[pt]",
	StudyTypeObservational: "Observational Study[pt]",
	StudyTypeClinicalTrial: "Clinical Trial[pt]",
}

// PubTypeFilterTerm returns the PubMed filter term for a StudyType.
// Returns "" if unrecognised (safe to include in a query—caller can skip empty strings).
func PubTypeFilterTerm(st StudyType) string {
	return pubTypeFilter[st]
}

// Filters holds optional query constraints applied by the planner.
type Filters struct {
	StudyTypes []StudyType `json:"study_types,omitempty"`
	DateFrom   string      `json:"date_from,omitempty"` // YYYY/MM/DD
	DateTo     string      `json:"date_to,omitempty"`   // YYYY/MM/DD
	Languages  []string    `json:"languages,omitempty"` // ISO 639-1 codes, e.g. "en"
	HumansOnly bool        `json:"humans_only,omitempty"`
}

// QueryPlan is the structured output of the plan_pubmed_query tool.
type QueryPlan struct {
	BooleanQuery string    `json:"boolean_query"`
	MeshTerms    []string  `json:"mesh_terms,omitempty"`
	Filters      Filters   `json:"filters,omitempty"`
	SortOrder    SortOrder `json:"sort_order,omitempty"`
	Rationale    string    `json:"rationale,omitempty"`
	MaxResults   int       `json:"max_results,omitempty"` // 0 means use default (100)
}

// SearchResult is the output of a successful esearch call.
type SearchResult struct {
	PMIDs       []string `json:"pmids"`
	TotalCount  int      `json:"total_count"`
	QueryEchoed string   `json:"query_echoed,omitempty"`
	WebEnv      string   `json:"web_env,omitempty"`
	QueryKey    string   `json:"query_key,omitempty"`
}

// Article holds the metadata and abstract for a single PubMed article.
type Article struct {
	PMID            string   `json:"pmid"`
	Title           string   `json:"title"`
	Journal         string   `json:"journal"`
	PublicationDate string   `json:"publication_date"` // best-effort YYYY-MM-DD or YYYY
	Abstract        string   `json:"abstract,omitempty"`
	Authors         []string `json:"authors,omitempty"`
	MeSH            []string `json:"mesh,omitempty"`
	DOI             string   `json:"doi,omitempty"`
}
