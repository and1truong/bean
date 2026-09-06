// Package content defines Bean's bounded semantic content vocabulary.
package content

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/beanruntime/bean/internal/appir"
)

const (
	MaxElements         = 12
	MaxBulletItems      = 6
	MaxDiagramItems     = 8
	MaxCodeLines        = 120
	MaxOrderedItems     = 6
	MaxItemRunes        = 240
	MaxLabelRunes       = 120
	MaxTargetRunes      = 2048
	MaxColumns          = 6
	MaxRows             = 12
	MaxColumnLabelRunes = 80
	MaxMediaTitleRunes  = 120
	MaxTranscriptRunes  = 4000
	MinChoices          = 2
	MaxChoices          = 6
	MaxQuestionRunes    = 240
	MaxChoiceTextRunes  = 120
	MaxExplanationRunes = 400
	MaxMachineIDRunes   = 64
	MinTabs             = 2
	MaxTabs             = 6
	MaxTabElements      = 24
)

var machineID = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var playlistID = regexp.MustCompile(`^[A-Za-z0-9_-]{10,80}$`)

func Types() []string {
	return []string{"audio", "bullets", "callout", "choices", "code", "diagram", "divider", "heading", "image", "link", "ordered_list", "paragraph", "quote", "table", "youtube", "youtube_playlist"}
}

func Tones() []string { return []string{"info", "success", "warning"} }

func Directions() []string { return []string{"horizontal", "vertical"} }

func HeadingLevels() []int { return []int{2, 3, 4} }

func LinkOpenModes() []string { return []string{"new_tab", "same_tab"} }

func RowHeaderModes() []string { return []string{"first", "none"} }

func TabOrientations() []string { return []string{"horizontal", "vertical"} }

func TabVariants() []string { return []string{"pills", "underline"} }

func ValidMachineID(value string) bool {
	return utf8.RuneCountInString(value) <= MaxMachineIDRunes && machineID.MatchString(value)
}

func ValidVideoID(value string) bool { return videoID.MatchString(value) }

func ValidPlaylistID(value string) bool { return playlistID.MatchString(value) }

func Normalize(elements []appir.ContentElement) {
	for index := range elements {
		if elements[index].Type == "callout" && elements[index].Tone == "" {
			elements[index].Tone = "info"
		}
		if elements[index].Type == "diagram" && elements[index].Direction == "" {
			elements[index].Direction = "horizontal"
		}
		if elements[index].Type == "heading" && elements[index].Level == 0 {
			elements[index].Level = 2
		}
		if elements[index].Type == "link" && elements[index].OpenIn == "" {
			elements[index].OpenIn = "same_tab"
		}
		if elements[index].Type == "table" && elements[index].RowHeader == "" {
			elements[index].RowHeader = "none"
		}
		if elements[index].Type == "choices" && strings.TrimSpace(elements[index].Explanation) == "" {
			elements[index].Explanation = ""
		}
	}
}

func Weight(elements []appir.ContentElement) int {
	total := 0
	for _, element := range elements {
		total += utf8.RuneCountInString(element.Text) + utf8.RuneCountInString(element.Attribution)
		for _, item := range element.Items {
			total += utf8.RuneCountInString(item)
		}
		switch element.Type {
		case "bullets":
			total += len(element.Items) * 20
		case "code":
			total += strings.Count(element.Text, "\n") * 12
		case "image":
			total += 180
		case "diagram":
			total += 60 + len(element.Items)*40
		case "ordered_list":
			total += len(element.Items) * 20
		case "link":
			total += utf8.RuneCountInString(element.Label) + 20
		case "divider":
			total += 20
		case "table":
			total += utf8.RuneCountInString(element.Caption) + 20*len(element.Columns) + 20*len(element.Rows)
			for _, column := range element.Columns {
				total += utf8.RuneCountInString(column.Label)
			}
			for _, row := range element.Rows {
				for _, cell := range row {
					total += utf8.RuneCountInString(cell)
				}
			}
		case "audio", "youtube", "youtube_playlist":
			total += utf8.RuneCountInString(element.Title) + utf8.RuneCountInString(element.Transcript) + 180
		case "choices":
			total += utf8.RuneCountInString(element.Question) + utf8.RuneCountInString(element.Explanation) + len(element.Choices)*20
			for _, choice := range element.Choices {
				total += utf8.RuneCountInString(choice.Text)
			}
		}
	}
	return total
}

func ValidLinkTarget(raw string) bool { return validURL(raw, true) }

func ValidAudioSource(raw string) bool {
	return !strings.ContainsAny(raw, "?#") && validURL(raw, false)
}

func validURL(raw string, allowQueryFragment bool) bool {
	if raw == "" || strings.TrimSpace(raw) != raw || utf8.RuneCountInString(raw) > MaxTargetRunes || strings.Contains(raw, "\\") {
		return false
	}
	for _, character := range raw {
		if unicode.IsControl(character) {
			return false
		}
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil || strings.Contains(decoded, "\\") {
		return false
	}
	for _, character := range decoded {
		if unicode.IsControl(character) {
			return false
		}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Opaque != "" || (!allowQueryFragment && (parsed.RawQuery != "" || parsed.Fragment != "")) {
		return false
	}
	if parsed.Scheme == "https" {
		if parsed.Host == "" || parsed.Hostname() == "" {
			return false
		}
	} else if parsed.Scheme != "" || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return false
	}
	pathValue, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return false
	}
	for _, segment := range strings.Split(pathValue, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func ValidImageSource(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return parsed.Host != "" && parsed.Hostname() != ""
	}
	return parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") && !strings.HasPrefix(parsed.Path, "//") && !strings.Contains(parsed.Path, "..") && !strings.Contains(parsed.Path, "\\")
}
