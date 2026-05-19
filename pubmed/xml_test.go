package pubmed

import (
	"os"
	"testing"
)

func TestParseEFetchXML(t *testing.T) {
	data, err := os.ReadFile("testdata/efetch_sample.xml")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	articles, err := ParseEFetchXML(data)
	if err != nil {
		t.Fatalf("ParseEFetchXML: %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("got %d articles, want 2", len(articles))
	}

	a := articles[0]
	if a.PMID != "12345678" {
		t.Errorf("articles[0].PMID = %q, want 12345678", a.PMID)
	}
	if a.Title != "Aspirin and Cardiovascular Events after STEMI: A Randomized Trial" {
		t.Errorf("articles[0].Title = %q (unexpected)", a.Title)
	}
	if a.Journal != "N Engl J Med" {
		t.Errorf("articles[0].Journal = %q, want N Engl J Med", a.Journal)
	}
	if a.PublicationDate != "2023-11-16" {
		t.Errorf("articles[0].PublicationDate = %q, want 2023-11-16", a.PublicationDate)
	}
	if len(a.Authors) != 2 {
		t.Errorf("articles[0].Authors len = %d, want 2", len(a.Authors))
	}
	if a.DOI != "10.1056/NEJMoa2300001" {
		t.Errorf("articles[0].DOI = %q", a.DOI)
	}
	if len(a.MeSH) != 2 {
		t.Errorf("articles[0].MeSH len = %d, want 2", len(a.MeSH))
	}
	if a.Abstract == "" {
		t.Error("articles[0].Abstract is empty")
	}

	b := articles[1]
	if b.PMID != "87654321" {
		t.Errorf("articles[1].PMID = %q, want 87654321", b.PMID)
	}
	if b.PublicationDate != "2022" {
		t.Errorf("articles[1].PublicationDate = %q, want 2022", b.PublicationDate)
	}
	if len(b.Authors) != 1 || b.Authors[0] != "Antiplatelet Trialists Collaboration" {
		t.Errorf("articles[1].Authors = %v", b.Authors)
	}
}

func TestFormatPubDate(t *testing.T) {
	tests := []struct {
		d    xmlPubDate
		want string
	}{
		{xmlPubDate{Year: "2023", Month: "Nov", Day: "16"}, "2023-11-16"},
		{xmlPubDate{Year: "2023", Month: "3"}, "2023-03"},
		{xmlPubDate{Year: "2022"}, "2022"},
		{xmlPubDate{MedlineDate: "2021 Jul-Aug"}, "2021"},
		{xmlPubDate{}, ""},
	}
	for _, tc := range tests {
		got := formatPubDate(tc.d)
		if got != tc.want {
			t.Errorf("formatPubDate(%+v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestBuildSearchTerm(t *testing.T) {
	plan := QueryPlan{
		BooleanQuery: "aspirin[MeSH]",
		Filters: Filters{
			StudyTypes: []StudyType{StudyTypeRCT},
			DateFrom:   "2020/01/01",
			DateTo:     "2023/12/31",
			HumansOnly: true,
		},
	}
	term := buildSearchTerm(plan)

	checks := []string{
		"aspirin[MeSH]",
		"Randomized Controlled Trial[pt]",
		`"2020/01/01"[PDAT]`,
		`"2023/12/31"[PDAT]`,
		"humans[MeSH Terms]",
	}
	for _, c := range checks {
		if !contains(term, c) {
			t.Errorf("buildSearchTerm: missing %q in %q", c, term)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
