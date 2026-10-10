package translate

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestTotalTextLength(t *testing.T) {
	tests := []struct {
		name      string
		texts     []string
		wantTotal int
		wantOK    bool
	}{
		{"no texts", nil, 0, false},
		{"empty array", []string{}, 0, false},
		{"empty string", []string{""}, 0, false},
		{"empty element in batch", []string{"Hello", ""}, 0, false},
		{"single text", []string{"Hello"}, 5, true},
		{"batch total", []string{"Hello", "Good morning"}, 17, true},
		{"counts runes, not bytes", []string{"你好"}, 2, true},
		{"at the limit", []string{strings.Repeat("a", maxFreeTextLength)}, maxFreeTextLength, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, ok := totalTextLength(tt.texts)
			if ok != tt.wantOK || total != tt.wantTotal {
				t.Fatalf("totalTextLength(%q) = (%d, %v), want (%d, %v)", tt.texts, total, ok, tt.wantTotal, tt.wantOK)
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
		{"empty element in batch", []string{"Hello", ""}, http.StatusNotFound},
		{"over the limit", []string{strings.Repeat("a", maxFreeTextLength+1)}, http.StatusRequestEntityTooLarge},
		{"batch over the limit", []string{strings.Repeat("a", maxFreeTextLength), "b"}, http.StatusRequestEntityTooLarge},
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
