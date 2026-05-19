package pubmed

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// ParseEFetchXML parses a PubMed efetch XML response and returns a slice of Articles.
// Missing fields are silently set to empty string to avoid hard failures on schema variations.
func ParseEFetchXML(data []byte) ([]Article, error) {
	var set pubmedArticleSet
	if err := xml.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("parsing efetch XML: %w", err)
	}

	articles := make([]Article, 0, len(set.Articles))
	for _, raw := range set.Articles {
		articles = append(articles, toArticle(raw))
	}
	return articles, nil
}

// pubmedArticleSet mirrors the PubmedArticleSet root element.
type pubmedArticleSet struct {
	XMLName  xml.Name         `xml:"PubmedArticleSet"`
	Articles []pubmedArticle  `xml:"PubmedArticle"`
}

type pubmedArticle struct {
	MedlineCitation medlineCitation `xml:"MedlineCitation"`
	PubmedData      pubmedData      `xml:"PubmedData"`
}

type medlineCitation struct {
	PMID    pmidField   `xml:"PMID"`
	Article xmlArticle  `xml:"Article"`
	MeshHeadingList struct {
		Headings []struct {
			Descriptor struct {
				Name string `xml:",chardata"`
			} `xml:"DescriptorName"`
		} `xml:"MeshHeading"`
	} `xml:"MeshHeadingList"`
}

type pmidField struct {
	Value string `xml:",chardata"`
}

type xmlArticle struct {
	ArticleTitle string      `xml:"ArticleTitle"`
	Journal      xmlJournal  `xml:"Journal"`
	Abstract     xmlAbstract `xml:"Abstract"`
	AuthorList   struct {
		Authors []xmlAuthor `xml:"Author"`
	} `xml:"AuthorList"`
	ELocationIDList struct {
		IDs []struct {
			Type  string `xml:"EIdType,attr"`
			Value string `xml:",chardata"`
		} `xml:"ELocationID"`
	} `xml:"ELocationIDList"`
}

type xmlJournal struct {
	Title     string         `xml:"Title"`
	ISOAbbrev string         `xml:"ISOAbbreviation"`
	JournalIssue xmlJournalIssue `xml:"JournalIssue"`
}

type xmlJournalIssue struct {
	PubDate xmlPubDate `xml:"PubDate"`
}

type xmlPubDate struct {
	Year   string `xml:"Year"`
	Month  string `xml:"Month"`
	Day    string `xml:"Day"`
	MedlineDate string `xml:"MedlineDate"`
}

type xmlAbstract struct {
	Texts []struct {
		Label string `xml:"Label,attr"`
		Type  string `xml:"NlmCategory,attr"`
		Text  string `xml:",chardata"`
	} `xml:"AbstractText"`
}

type xmlAuthor struct {
	LastName    string `xml:"LastName"`
	ForeName    string `xml:"ForeName"`
	Initials    string `xml:"Initials"`
	CollectiveName string `xml:"CollectiveName"`
}

type pubmedData struct {
	ArticleIDList struct {
		IDs []struct {
			Type  string `xml:"IdType,attr"`
			Value string `xml:",chardata"`
		} `xml:"ArticleId"`
	} `xml:"ArticleIdList"`
}

func toArticle(raw pubmedArticle) Article {
	a := Article{
		PMID:  raw.MedlineCitation.PMID.Value,
		Title: cleanTitle(raw.MedlineCitation.Article.ArticleTitle),
	}

	j := raw.MedlineCitation.Article.Journal
	if j.ISOAbbrev != "" {
		a.Journal = j.ISOAbbrev
	} else {
		a.Journal = j.Title
	}

	a.PublicationDate = formatPubDate(j.JournalIssue.PubDate)

	a.Abstract = buildAbstract(raw.MedlineCitation.Article.Abstract)

	for _, auth := range raw.MedlineCitation.Article.AuthorList.Authors {
		if auth.CollectiveName != "" {
			a.Authors = append(a.Authors, auth.CollectiveName)
		} else if auth.LastName != "" {
			name := auth.LastName
			if auth.Initials != "" {
				name += " " + auth.Initials
			}
			a.Authors = append(a.Authors, name)
		}
	}

	for _, h := range raw.MedlineCitation.MeshHeadingList.Headings {
		if h.Descriptor.Name != "" {
			a.MeSH = append(a.MeSH, h.Descriptor.Name)
		}
	}

	for _, id := range raw.PubmedData.ArticleIDList.IDs {
		if id.Type == "doi" {
			a.DOI = id.Value
		}
	}

	return a
}

func cleanTitle(s string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	return s
}

func formatPubDate(d xmlPubDate) string {
	if d.Year != "" {
		if d.Month != "" && d.Day != "" {
			return fmt.Sprintf("%s-%s-%s", d.Year, normalizeMonth(d.Month), normalizeDay(d.Day))
		}
		if d.Month != "" {
			return fmt.Sprintf("%s-%s", d.Year, normalizeMonth(d.Month))
		}
		return d.Year
	}
	if d.MedlineDate != "" {
		// MedlineDate is free-text like "2023 Jan-Feb"; extract year.
		parts := strings.Fields(d.MedlineDate)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}

var monthMap = map[string]string{
	"Jan": "01", "Feb": "02", "Mar": "03", "Apr": "04",
	"May": "05", "Jun": "06", "Jul": "07", "Aug": "08",
	"Sep": "09", "Oct": "10", "Nov": "11", "Dec": "12",
}

func normalizeMonth(m string) string {
	if num, ok := monthMap[m]; ok {
		return num
	}
	// Already numeric (e.g. "3" or "03")
	if len(m) == 1 {
		return "0" + m
	}
	return m
}

func normalizeDay(d string) string {
	if len(d) == 1 {
		return "0" + d
	}
	return d
}

func buildAbstract(abs xmlAbstract) string {
	if len(abs.Texts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(abs.Texts))
	for _, t := range abs.Texts {
		text := strings.TrimSpace(t.Text)
		if text == "" {
			continue
		}
		if t.Label != "" {
			parts = append(parts, t.Label+": "+text)
		} else {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}
