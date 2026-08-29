package render

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"time"

	"github.com/beanruntime/bean/internal/dbal"
)

type Node struct {
	Component string         `json:"component"`
	Props     map[string]any `json:"props,omitempty"`
	Children  []Node         `json:"children,omitempty"`
}

func JSON(rows []dbal.Row) ([]byte, error) { return json.Marshal(map[string]any{"data": rows}) }
func CSV(rows []dbal.Row) ([]byte, error) {
	keys := columns(rows)
	b := bytes.Buffer{}
	w := csv.NewWriter(&b)
	_ = w.Write(keys)
	for _, r := range rows {
		line := make([]string, len(keys))
		for i, k := range keys {
			line[i] = fmt.Sprint(r[k])
		}
		_ = w.Write(line)
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

type rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel channel  `xml:"channel"`
}
type channel struct {
	Title, Link, Description string
	Items                    []item `xml:"item"`
}
type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link,omitempty"`
	Description string `xml:"description,omitempty"`
	GUID        string `xml:"guid,omitempty"`
	PubDate     string `xml:"pubDate,omitempty"`
}

func RSS(title, link string, rows []dbal.Row) ([]byte, error) {
	items := []item{}
	for _, r := range rows {
		it := item{Title: first(r, "title", "name", "id"), GUID: fmt.Sprint(r["id"]), Description: first(r, "summary", "body", "description")}
		if v := r["published_at"]; v != nil {
			if t, e := time.Parse(time.RFC3339, fmt.Sprint(v)); e == nil {
				it.PubDate = t.Format(time.RFC1123Z)
			}
		}
		items = append(items, it)
	}
	b, e := xml.Marshal(rss{Version: "2.0", Channel: channel{Title: title, Link: link, Description: title, Items: items}})
	return append([]byte(xml.Header), b...), e
}
func columns(rows []dbal.Row) []string {
	set := map[string]bool{}
	for _, r := range rows {
		for k := range r {
			set[k] = true
		}
	}
	out := []string{}
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func first(r dbal.Row, keys ...string) string {
	for _, k := range keys {
		if r[k] != nil {
			return fmt.Sprint(r[k])
		}
	}
	return ""
}
