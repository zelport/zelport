package feature

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusAnswersAPIPathsWithJSONWithoutAccept(t *testing.T) {
	w := httptest.NewRecorder()
	Status(w, httptest.NewRequest("GET", "/api/state", nil), 503, "build-server is unreachable", "Relay is disconnected.")
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content type = %q, want JSON for an API path", w.Header().Get("Content-Type"))
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "build-server is unreachable" || body["detail"] != "Relay is disconnected." {
		t.Fatalf("body = %v", body)
	}
	page := httptest.NewRecorder()
	Status(page, httptest.NewRequest("GET", "/", nil), 503, "build-server is unreachable", "Relay is disconnected.")
	if !strings.HasPrefix(page.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("a page request keeps the HTML status page, got %q", page.Header().Get("Content-Type"))
	}
}
