package content_test

import (
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beancontent "github.com/beanruntime/bean/internal/content"
)

func TestSafeContentURLs(t *testing.T) {
	for _, target := range []string{"/presentations/bean?frame=architecture#detail", "/", "https://example.test/path?q=1#result", "https://例え.test/資料"} {
		if !beancontent.ValidLinkTarget(target) {
			t.Errorf("valid link rejected: %q", target)
		}
	}
	for _, target := range []string{"", " //example.test", "/trailing ", "//example.test/path", "http://example.test", "javascript:alert(1)", "data:text/plain,x", "blob:https://example.test/id", "file:///tmp/file", "https://user@example.test/a", "/a\\b", "/a%5Cb", "/a%5cb", "/a%00b", "/a%0Ab", "/a/%2e%2e/b", "/a/%2E/b", "/a%2f..%2fb", "/a/%zz", "/a/./b", "/a/../b", "https://example.test/%zz", "https://example.test/a?x=%0A"} {
		if beancontent.ValidLinkTarget(target) {
			t.Errorf("hostile link accepted: %q", target)
		}
	}
	for _, source := range []string{"/media/intro.mp3", "https://media.example.test/intro.mp3"} {
		if !beancontent.ValidAudioSource(source) {
			t.Errorf("valid audio rejected: %q", source)
		}
	}
	for _, source := range []string{"/media/intro.mp3?", "/media/intro.mp3#", "https://media.example.test/intro.mp3?q=1", "https://media.example.test/intro.mp3#part"} {
		if beancontent.ValidAudioSource(source) {
			t.Errorf("audio query or fragment accepted: %q", source)
		}
	}
}

func TestExtendedContentWeights(t *testing.T) {
	elements := []appir.ContentElement{
		{Type: "ordered_list", Items: []string{"ab", "c"}},
		{Type: "link", Label: "open"},
		{Type: "divider"},
		{Type: "choices", Question: "Q", Choices: []appir.ContentChoice{{Text: "A"}, {Text: "BC"}}, Explanation: "E"},
	}
	if got, want := beancontent.Weight(elements), 3+40+4+20+20+1+1+1+2+40; got != want {
		t.Fatalf("weight=%d want=%d", got, want)
	}
}

func TestYouTubeIdentifiersAreSyntaxOnly(t *testing.T) {
	for _, id := range []string{"M7lc1UVf-VE", "___________", "abcdefghijk"} {
		if !beancontent.ValidVideoID(id) {
			t.Errorf("valid video ID rejected: %q", id)
		}
	}
	for _, id := range []string{"short", "M7lc1UVf-VE?autoplay=1", "M7lc1UVf VE", "M7lc1UVf:VE"} {
		if beancontent.ValidVideoID(id) {
			t.Errorf("invalid video ID accepted: %q", id)
		}
	}
	for _, id := range []string{"ABCDEFGHIJ", "PL1234567890ABCDEFG", strings.Repeat("A", 80)} {
		if !beancontent.ValidPlaylistID(id) {
			t.Errorf("valid playlist ID rejected: %q", id)
		}
	}
	for _, id := range []string{"123456789", strings.Repeat("A", 81), "https://youtube.test/playlist", "ABCDEFGHIJ?x=1", "ABCDEFGHIJ:"} {
		if beancontent.ValidPlaylistID(id) {
			t.Errorf("invalid playlist ID accepted: %q", id)
		}
	}
}
