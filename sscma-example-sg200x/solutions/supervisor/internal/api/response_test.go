package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteSuccess_WritesStandardEnvelope(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	if err := WriteSuccess(rec, map[string]interface{}{"k": "v"}); err != nil {
		t.Fatalf("WriteSuccess returned error: %v", err)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content-type: %q", ct)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var r Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}
	if r.Code != 0 || r.Message != "OK" {
		t.Fatalf("unexpected envelope: %+v", r)
	}
	data, ok := r.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", r.Data)
	}
	if data["k"] != "v" {
		t.Fatalf("unexpected data: %#v", data)
	}
}
