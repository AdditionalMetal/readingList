// +build mage

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"text/template"
	"time"

	"github.com/jszwec/csvutil"
	"github.com/stevelacy/daz"
)

const dateFormat = "2006-01-02"

type readingListEntry struct {
	URL         string    `csv:"url,omitempty"`
	Title       string    `csv:"title,omitempty"`
	Description string    `csv:"description,omitempty"`
	Image       string    `csv:"image,omitempty"`
	Date        time.Time `csv:"date,omitempty"`
}

// renderAnchor renders a HTML anchor tag
func renderAnchor(text, url string, newTab bool) daz.HTML {
	attrs := daz.Attr{
		"href": url,
		"rel":  "noopener",
	}
	if newTab {
		attrs["target"] = "_blank"
	}
	return daz.H("a", attrs, text)
}

func renderUnsafeAnchor(text, url string, newTab bool) daz.HTML {
	attrs := daz.Attr{
		"href": url,
		"rel":  "noopener",
	}
	if newTab {
		attrs["target"] = "_blank"
	}
	return daz.H("a", attrs, daz.UnsafeContent(text))
}

//go:embed page.template.html
var htmlPageTemplate []byte

// renderHTMLPage renders a complete HTML page
func renderHTMLPage(title, titleBar, pageContent, extraHeadeContent string) ([]byte, error) {

	tpl, err := template.New("page").Parse(string(htmlPageTemplate))
	if err != nil {
		return nil, err
	}
	outputBuf := new(bytes.Buffer)

	tpl.Execute(outputBuf, struct {
		Title            string
		Content          string
		PageTitleBar     string
		ExtraHeadContent string
	}{Content: pageContent, PageTitleBar: titleBar, Title: title, ExtraHeadContent: extraHeadeContent})

	return outputBuf.Bytes(), nil
}

type entryGroup struct {
	Date    time.Time
	Entries entrySlice
}

type entryGroupSlice []*entryGroup

func (e entryGroupSlice) Len() int {
	return len(e)
}

func (e entryGroupSlice) Less(i, j int) bool {
	return e[i].Date.After(e[j].Date)
}

func (e entryGroupSlice) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

type entrySlice []*readingListEntry

func (e entrySlice) Len() int {
	return len(e)
}

func (e entrySlice) Less(i, j int) bool {
	return e[i].Date.After(e[j].Date)
}

func (e entrySlice) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func groupEntriesByMonth(entries []*readingListEntry) entryGroupSlice {
	groupMap := make(map[time.Time]*entryGroup)

	for _, entry := range entries {
		newTime := time.Date(entry.Date.Year(), entry.Date.Month(), 1, 0, 0, 0, 0, time.UTC)
		if groupMap[newTime] == nil {
			groupMap[newTime] = &entryGroup{
				Date: newTime,
			}
		}
		groupMap[newTime].Entries = append(groupMap[newTime].Entries, entry)
	}

	var o entryGroupSlice
	for _, group := range groupMap {
		sort.Sort(group.Entries)
		o = append(o, group)
	}
	sort.Sort(o)

	return o
}

// makeTILHTML generates HTML from a []*entryGroup to make a list of articles
func makeListHTML(groups []*entryGroup) string {

	const headerLevel = "h2"

	var parts []interface{}
	for _, group := range groups {

		header := daz.H(headerLevel, fmt.Sprintf("%s %d", group.Date.Month().String(), group.Date.Year()))

		var entries []daz.HTML
		for _, article := range group.Entries {

			titleLine := daz.H("summary", renderAnchor(article.Title, article.URL, false), " - " + article.Date.Format(dateFormat))

			detailedInfo := []interface{}{}

			{
				var descriptionContent string
				if article.Description != "" {
					descriptionContent = article.Description
				} else {
					descriptionContent = "<none>"
				}
				detailedInfo = append(detailedInfo, daz.H("div", "Description:", daz.H("i", descriptionContent)))
			}

			{
				if article.Image != "" {
					detailedInfo = append(detailedInfo, daz.H("div", "Image:", daz.H("br"), daz.H("img", daz.Attr{"src": article.Image, "loading": "lazy", "style": "max-width: 256px;"})))
				}
			}

			detailedInfo = append(detailedInfo, daz.Attr{"class": "description"})
			entries = append(entries, daz.H("li", daz.H("details", titleLine, daz.H("div", detailedInfo...))))
		}

		parts = append(parts, []daz.HTML{header, daz.H("ul", entries)})
	}

	return daz.H("div", parts...)()
}

func GenerateSite() error {

	const outputDir = ".site"
	const readingListFile = "readingList.csv"

	// read CSV file
	var entries []*readingListEntry

	fcont, err := ioutil.ReadFile(readingListFile)
	if err != nil {
		return err
	}

	err = csvutil.Unmarshal(fcont, &entries)
	if err != nil {
		return err
	}

	numArticles := len(entries)
	groupedEntries := groupEntriesByMonth(entries)

	const pageTitle = "akp's reading list"

	head := daz.H(
		"div",
		daz.Attr{"class": "heading"},
		daz.H("h1", pageTitle),
		daz.H(
			"p",
			daz.Attr{"class": "information"},
			daz.UnsafeContent(
				fmt.Sprintf(
					"A mostly complete list of articles I've read<br>There are currently %d entries in the list<br>Last modified %s<br>Repo: %s",
					numArticles,
					time.Now().Format(dateFormat),
					renderUnsafeAnchor("<code>codemicro/readingList</code>", "https://github.com/codemicro/readingList", false)(),
				),
			),
		),
	)

	listingHTML := makeListHTML(groupedEntries)

	outputContent, err := renderHTMLPage(pageTitle, head(), listingHTML, "")
	if err != nil {
		return err
	}

	_ = os.Mkdir(".site", 0777)

	return ioutil.WriteFile(".site/index.html", outputContent, 0644)
}
