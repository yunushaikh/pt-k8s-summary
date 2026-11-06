package collector

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPreviewPodLogKeepsNewestLines(t *testing.T) {
	var b strings.Builder
	b.WriteString("OLDEST-LINE-MARKER\n")
	for i := 0; i < 20000; i++ {
		b.WriteString("old line ")
		b.WriteString(strings.Repeat("x", 40))
		b.WriteByte('\n')
	}
	b.WriteString("NEWEST-LINE-MARKER\n")
	raw := b.String()
	if len(raw) <= pxcLogMaxBytes {
		t.Fatalf("fixture too small: %d", len(raw))
	}
	got, trunc := previewPodLog([]byte(raw))
	if !trunc {
		t.Fatal("expected truncation")
	}
	if strings.Contains(got, "OLDEST-LINE-MARKER") {
		t.Fatal("preview still contains the oldest line")
	}
	if !strings.Contains(got, "NEWEST-LINE-MARKER") {
		t.Fatal("preview dropped the newest line")
	}
	if !strings.HasPrefix(got, "… [newest lines") {
		t.Fatalf("missing newest-lines banner: %q", got[:80])
	}
	if !utf8.ValidString(got) {
		t.Fatal("preview is not valid UTF-8")
	}
}

func TestPreviewPodLogLeavesSmallFileWhole(t *testing.T) {
	raw := "first\nlast\n"
	got, trunc := previewPodLog([]byte(raw))
	if trunc || got != raw {
		t.Fatalf("got trunc=%v %q", trunc, got)
	}
}

func TestTailBytesSkipsTornRune(t *testing.T) {
	// € is 3 bytes. Cut so the window would start mid-rune without the boundary check.
	body := strings.Repeat("a", 20) + "€" + strings.Repeat("b", 20)
	got := string(tailBytes([]byte(body), 22))
	if !utf8.ValidString(got) {
		t.Fatalf("invalid utf-8 tail %q", got)
	}
	if strings.Contains(got, "a") {
		t.Fatalf("tail included the old prefix: %q", got)
	}
}
