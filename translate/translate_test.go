package translate

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestTextsToTranslate(t *testing.T) {
	tests := []struct {
		name      string
		texts     []string
		wantAt    []int
		wantTotal int
		wantText  bool
	}{
		{"no texts", nil, nil, 0, false},
		{"empty array", []string{}, nil, 0, false},
		{"empty string", []string{""}, nil, 0, false},
		{"every text empty", []string{"", ""}, nil, 0, false},
		{"whitespace only", []string{"  \t "}, nil, 0, true},
		{"empty and blank", []string{"", "  "}, nil, 0, true},
		{"single text", []string{"Hello"}, []int{0}, 5, true},
		{"empty element in batch", []string{"Hello", ""}, []int{0}, 5, true},
		{"leading empty element in batch", []string{"", "Hello"}, []int{1}, 5, true},
		{"blank elements around a batch", []string{"", "Hello", "  ", "Good morning", "\n"}, []int{1, 3}, 17, true},
		{"batch total", []string{"Hello", "Good morning"}, []int{0, 1}, 17, true},
		{"counts UTF-16 units, not bytes", []string{"你好"}, []int{0}, 2, true},
		{"astral character costs two units", []string{"😀"}, []int{0}, 2, true},
		{"combining mark is two units, not one grapheme", []string{"e\u0301"}, []int{0}, 2, true},
		{"invalid UTF-8 counts one unit per byte", []string{"\xf0\x90\x80"}, []int{0}, 3, true},
		{"at the limit", []string{strings.Repeat("a", maxFreeTextLength)}, []int{0}, maxFreeTextLength, true},
		{"astral at the limit", []string{strings.Repeat("😀", maxFreeTextLength/2)}, []int{0}, maxFreeTextLength, true},
		{"astral past the limit", []string{strings.Repeat("😀", maxFreeTextLength/2), "😀"}, []int{0, 1}, maxFreeTextLength + 2, true},
		{"astral batch sums units, not runes", []string{strings.Repeat("😀", 400), strings.Repeat("😀", 400)}, []int{0, 1}, 1600, true},
		{"blank texts are not charged", []string{"   ", "Hello"}, []int{1}, 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at, total, hasText := textsToTranslate(tt.texts)
			if hasText != tt.wantText || total != tt.wantTotal || !reflect.DeepEqual(at, tt.wantAt) {
				t.Fatalf("textsToTranslate(%q) = (%v, %d, %v), want (%v, %d, %v)", tt.texts, at, total, hasText, tt.wantAt, tt.wantTotal, tt.wantText)
			}
		})
	}
}

func TestMergeTranslations(t *testing.T) {
	tests := []struct {
		name         string
		texts        []string
		positions    []int
		translations []string
		want         []string
	}{
		{"no blanks", []string{"Hello"}, []int{0}, []string{"你好"}, []string{"你好"}},
		{"leading blank", []string{"", "Hello"}, []int{1}, []string{"你好"}, []string{"", "你好"}},
		{"blank keeps its exact text", []string{"", "Hello", "  "}, []int{1}, []string{"你好"}, []string{"", "你好", "  "}},
		{"batch keeps order", []string{"Hello", "  ", "Good morning"}, []int{0, 2}, []string{"你好", "早上好"}, []string{"你好", "  ", "早上好"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeTranslations(tt.texts, tt.positions, tt.translations); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("mergeTranslations(%q, %v, %q) = %q, want %q", tt.texts, tt.positions, tt.translations, got, tt.want)
			}
		})
	}
}

func TestTranslationsFromResult(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		count   int
		want    []string
		wantErr bool
	}{
		{
			name:  "single",
			body:  `{"translations":[{"text":"你好，世界！","detected_source_language":"en"}]}`,
			count: 1,
			want:  []string{"你好，世界！"},
		},
		{
			name:  "batch keeps order",
			body:  `{"translations":[{"text":"你好，世界！"},{"text":"早上好"}]}`,
			count: 2,
			want:  []string{"你好，世界！", "早上好"},
		},
		{name: "no translations at all", body: `{}`, count: 1, wantErr: true},
		{name: "fewer than requested", body: `{"translations":[{"text":"你好"}]}`, count: 2, wantErr: true},
		{name: "more than requested", body: `{"translations":[{"text":"你好"},{"text":"早上好"}]}`, count: 1, wantErr: true},
		{name: "empty translation", body: `{"translations":[{"text":"你好"},{"text":""}]}`, count: 2, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := translationsFromResult(gjson.Parse(tt.body), tt.count)
			if (err != nil) != tt.wantErr {
				t.Fatalf("translationsFromResult() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("translationsFromResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Rejected requests must fail before anything is sent upstream: every case
// below returns on validation alone, so the test does not touch DeepL.
func TestTranslateByDLXValidationFailsBeforeUpstream(t *testing.T) {
	tests := []struct {
		name  string
		texts []string
		code  int
	}{
		{"no texts", nil, http.StatusNotFound},
		{"empty array", []string{}, http.StatusNotFound},
		{"empty string", []string{""}, http.StatusNotFound},
		{"every text empty", []string{"", ""}, http.StatusNotFound},
		{"over the limit", []string{strings.Repeat("a", maxFreeTextLength+1)}, http.StatusRequestEntityTooLarge},
		{"batch over the limit", []string{strings.Repeat("a", maxFreeTextLength), "b"}, http.StatusRequestEntityTooLarge},
		// 751 runes in the batch, but 1502 UTF-16 units: the case a rune count
		// let through to be rejected by oneshot instead.
		{"astral batch over the unit limit", []string{strings.Repeat("😀", maxFreeTextLength/2+1)}, http.StatusRequestEntityTooLarge},
		{"astral batch split over the unit limit", []string{strings.Repeat("😀", 400), strings.Repeat("😀", 400)}, http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TranslateByDLX("", "ZH", tt.texts, "", "", "")
			if err != nil {
				t.Fatalf("TranslateByDLX() error = %v", err)
			}
			if result.Code != tt.code {
				t.Fatalf("TranslateByDLX() code = %d, want %d", result.Code, tt.code)
			}
		})
	}
}

// A request that has text but nothing to translate is answered with its own
// texts rather than rejected, and — like the validation cases above — it never
// reaches DeepL.
func TestTranslateByDLXAnswersBlankTextsWithThemselves(t *testing.T) {
	tests := [][]string{
		{"  \t "},
		{"", "  "},
		{"\n", "\t\n", "   "},
	}
	for _, texts := range tests {
		t.Run(strings.Join(texts, "|"), func(t *testing.T) {
			result, err := TranslateByDLX("", "ZH", texts, "", "", "")
			if err != nil {
				t.Fatalf("TranslateByDLX() error = %v", err)
			}
			if result.Code != http.StatusOK {
				t.Fatalf("TranslateByDLX() code = %d, want %d", result.Code, http.StatusOK)
			}
			if !reflect.DeepEqual(result.Data, texts) {
				t.Fatalf("TranslateByDLX() data = %q, want %q", result.Data, texts)
			}
		})
	}
}
