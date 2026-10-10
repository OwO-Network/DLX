package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
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
