package translate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

// The anonymous cap is a UTF-16 length, not a rune count: an astral character
// is a surrogate pair and costs two units, which is the only place the two
// disagree. A rune count understates such a request and lets oneshot answer it
// with a bare 400 instead.
func TestTextLength(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "Hello, world!", 13},
		{"BMP CJK", "你好", 2},
		{"highest BMP code point", "\uFFFF", 1},
		{"lowest astral code point", "\U00010000", 2},
		{"astral character", "😀", 2},
		{"astral characters", "😀😀", 4},
		{"mixed", "a😀b", 4},
		{"combining mark is two units, not one grapheme", "e\u0301", 2},
		{"at the limit", strings.Repeat("a", maxFreeTextLength), maxFreeTextLength},
		{"one past the limit", strings.Repeat("a", maxFreeTextLength+1), maxFreeTextLength + 1},
		{"astral at the limit", strings.Repeat("😀", maxFreeTextLength/2), maxFreeTextLength},
		{"astral past the limit", strings.Repeat("😀", maxFreeTextLength/2+1), maxFreeTextLength + 2},
		{"combining at the limit", strings.Repeat("e\u0301", maxFreeTextLength/2), maxFreeTextLength},
		{"combining past the limit", strings.Repeat("e\u0301", maxFreeTextLength/2+1), maxFreeTextLength + 2},
		{"invalid UTF-8 counts one unit per byte", "\xf0\x90\x80", 3},
		{"invalid UTF-8 mixed with ASCII", "a\xff\xfeb", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := textLength(tt.text); got != tt.want {
				t.Fatalf("textLength(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

// The guard counts JSON-decoded text, and encoding/json has already replaced
// every invalid byte with U+FFFD by then — json.Marshal would replace them
// again on the way upstream — so invalid UTF-8 never reaches textLength as
// invalid bytes, and the old rune count agreed with it exactly there. Invalid
// UTF-8 is therefore not what made the rune count wrong; an astral character
// is, which is the one case where the two columns below differ.
func TestTextLengthOnJSONDecodedText(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantText  string
		wantUnits int
		wantRunes int
	}{
		{"invalid UTF-8 in the body", "{\"text\":\"\xf0\x90\x80\"}", "\uFFFD\uFFFD\uFFFD", 3, 3},
		{"paired surrogate escape", `{"text":"\ud83d\ude00"}`, "😀", 2, 1},
		{"lone surrogate escape", `{"text":"\ud83d"}`, "\uFFFD", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(tt.body), &payload); err != nil {
				t.Fatalf("json.Unmarshal(%q) error = %v", tt.body, err)
			}
			if payload.Text != tt.wantText {
				t.Fatalf("decoded text = %q, want %q", payload.Text, tt.wantText)
			}
			if got := textLength(payload.Text); got != tt.wantUnits {
				t.Fatalf("textLength(%q) = %d, want %d", payload.Text, got, tt.wantUnits)
			}
			if got := utf8.RuneCountInString(payload.Text); got != tt.wantRunes {
				t.Fatalf("utf8.RuneCountInString(%q) = %d, want %d", payload.Text, got, tt.wantRunes)
			}
		})
	}
}

// Rejected requests must fail before anything is sent upstream: every case
// below returns on validation alone, so the test never reaches DeepL. The
// astral cases are the regression — 751 runes, which the old rune count
// accepted, but 1502 UTF-16 units, which oneshot answers with 400.
func TestTranslateByDLXRejectsOversizedTextsBeforeUpstream(t *testing.T) {
	tests := []struct {
		name  string
		texts []string
	}{
		{"one ASCII past the limit", []string{strings.Repeat("a", maxFreeTextLength+1)}},
		{"astral characters past the limit", []string{strings.Repeat("😀", maxFreeTextLength/2+1)}},
		{"combining marks past the limit", []string{strings.Repeat("e\u0301", maxFreeTextLength/2+1)}},
		{"invalid UTF-8 past the limit", []string{strings.Repeat("\xf0\x90\x80", maxFreeTextLength/3+1)}},
		// 800 runes in total, but two units each: a batch the rune count let
		// through in a single request.
		{"batch of astral characters past the limit", []string{strings.Repeat("😀", 400), strings.Repeat("😀", 400)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, length, ok := textsToTranslate(tt.texts)
			if !ok {
				t.Fatalf("textsToTranslate(%q) reported no text", tt.texts)
			}
			if length <= maxFreeTextLength {
				t.Fatalf("textsToTranslate(%q) = %d, which is not over the limit", tt.texts, length)
			}
			result, err := TranslateByDLX("", "ZH", tt.texts, "", "", "")
			if err != nil {
				t.Fatalf("TranslateByDLX() error = %v", err)
			}
			if result.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("TranslateByDLX() code = %d, want %d", result.Code, http.StatusRequestEntityTooLarge)
			}
			if want := fmt.Sprintf("%d", length); !strings.Contains(result.Message, want) {
				t.Fatalf("TranslateByDLX() message = %q, want it to name %s", result.Message, want)
			}
		})
	}
}
