package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// The `text` field of POST /translate accepts either shape; decoding uses the
// same encoding/json decoder gin binds request bodies with.
func TestPayloadTextUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		wantTexts []string
		wantBatch bool
		wantErr   bool
	}{
		{name: "string", payload: `{"text":"Hello"}`, wantTexts: []string{"Hello"}},
		{name: "empty string", payload: `{"text":""}`, wantTexts: []string{""}},
		{name: "array", payload: `{"text":["Hello","Good morning"]}`, wantTexts: []string{"Hello", "Good morning"}, wantBatch: true},
		{name: "single-element array", payload: `{"text":["Hello"]}`, wantTexts: []string{"Hello"}, wantBatch: true},
		{name: "empty array", payload: `{"text":[]}`, wantTexts: []string{}, wantBatch: true},
		{name: "null", payload: `{"text":null}`, wantTexts: []string{""}},
		{name: "no text field", payload: `{}`, wantTexts: nil},
		{name: "number", payload: `{"text":1}`, wantErr: true},
		{name: "object", payload: `{"text":{}}`, wantErr: true},
		{name: "mixed array", payload: `{"text":["Hello",1]}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req PayloadTranslate
			err := json.NewDecoder(strings.NewReader(tt.payload)).Decode(&req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decode(%s) error = %v, wantErr %v", tt.payload, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if req.Text.Batch != tt.wantBatch {
				t.Errorf("Batch = %v, want %v", req.Text.Batch, tt.wantBatch)
			}
			if !reflect.DeepEqual(req.Text.Texts, tt.wantTexts) {
				t.Errorf("Texts = %q, want %q", req.Text.Texts, tt.wantTexts)
			}
		})
	}
}

// POST /translate mirrors the shape it was given, and no shape that carries
// text is an error any more — an empty text is answered with itself and an
// empty array with an empty array. Every case here is answered (or refused)
// before a translation is attempted, so the test never reaches DeepL.
func TestTranslateHandlerAnswersEmptyText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := Router(&Config{})

	tests := []struct {
		name       string
		payload    string
		wantCode   int
		wantInBody string
	}{
		{"empty array", `{"text":[],"target_lang":"EN"}`, http.StatusOK, `"data":[]`},
		{"empty string", `{"text":"","target_lang":"EN"}`, http.StatusOK, `"data":""`},
		{"null", `{"text":null,"target_lang":"EN"}`, http.StatusOK, `"data":""`},
		{"array of empty", `{"text":[""],"target_lang":"EN"}`, http.StatusOK, `"data":[""]`},
		{"no text field", `{"target_lang":"EN"}`, http.StatusBadRequest, `Invalid request payload`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/translate", strings.NewReader(tt.payload))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantCode {
				t.Fatalf("POST /translate %s = %d %s, want %d", tt.payload, recorder.Code, recorder.Body.String(), tt.wantCode)
			}
			if !strings.Contains(recorder.Body.String(), tt.wantInBody) {
				t.Fatalf("POST /translate %s body = %s, want it to contain %s", tt.payload, recorder.Body.String(), tt.wantInBody)
			}
		})
	}
}
