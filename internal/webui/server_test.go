package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserAPIJourney(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow end-to-end Git test in -short mode")
	}
	server := httptest.NewServer((Server{Root: t.TempDir()}).Handler())
	defer server.Close()
	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("home status=%d", response.StatusCode)
	}
	_ = response.Body.Close()
	request := func(path, body string) *http.Response {
		t.Helper()
		r, err := http.Post(server.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	created := request("/api/start", `{"id":"add-add","attempt":"review"}`)
	if created.StatusCode != http.StatusOK {
		t.Fatalf("start status=%d", created.StatusCode)
	}
	var info attempt
	if err := json.NewDecoder(created.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	_ = created.Body.Close()
	if info.Folder != "add-add-review" {
		t.Fatalf("created folder=%q", info.Folder)
	}
	check := request("/api/check", `{"folder":"add-add-review"}`)
	if check.StatusCode != http.StatusOK {
		t.Fatalf("check status=%d", check.StatusCode)
	}
	var progress struct {
		Passed  bool
		Message string
	}
	if err := json.NewDecoder(check.Body).Decode(&progress); err != nil {
		t.Fatal(err)
	}
	_ = check.Body.Close()
	inspection := request("/api/inspect", `{"folder":"add-add-review"}`)
	if inspection.StatusCode != http.StatusOK {
		t.Fatalf("inspect status=%d", inspection.StatusCode)
	}
	var view struct {
		File string `json:"file"`
		Base struct {
			Exists bool `json:"exists"`
		} `json:"base"`
		Main struct {
			Content string `json:"content"`
		} `json:"main"`
		Feature struct {
			Content string `json:"content"`
		} `json:"feature"`
	}
	if err := json.NewDecoder(inspection.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	_ = inspection.Body.Close()
	if view.File != "notes.txt" || view.Base.Exists || !strings.Contains(view.Main.Content, "Main note") || !strings.Contains(view.Feature.Content, "Feature note") {
		t.Fatalf("unexpected inspection: %+v", view)
	}
	if progress.Passed || !strings.Contains(progress.Message, "git merge") {
		t.Fatalf("unexpected progress: %+v", progress)
	}
	hint := request("/api/hint", `{"folder":"add-add-review"}`)
	if hint.StatusCode != http.StatusOK {
		t.Fatalf("hint status=%d", hint.StatusCode)
	}
	_ = hint.Body.Close()
	bad := request("/api/check", `{"folder":"../elsewhere"}`)
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("path traversal status=%d", bad.StatusCode)
	}
	_ = bad.Body.Close()
}

func TestInspectMultipleConflictFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow end-to-end Git test in -short mode")
	}
	server := httptest.NewServer((Server{Root: t.TempDir()}).Handler())
	defer server.Close()
	post := func(path, body string) *http.Response {
		t.Helper()
		response, err := http.Post(server.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	created := post("/api/start", `{"id":"merge-multi"}`)
	if created.StatusCode != http.StatusOK {
		t.Fatalf("start status=%d", created.StatusCode)
	}
	_ = created.Body.Close()
	selected := post("/api/inspect", `{"folder":"merge-multi","file":"review.txt"}`)
	if selected.StatusCode != http.StatusOK {
		t.Fatalf("inspect status=%d", selected.StatusCode)
	}
	var view struct {
		File  string   `json:"file"`
		Files []string `json:"files"`
	}
	if err := json.NewDecoder(selected.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	_ = selected.Body.Close()
	if view.File != "review.txt" || len(view.Files) != 2 {
		t.Fatalf("inspection=%+v", view)
	}
	invalid := post("/api/inspect", `{"folder":"merge-multi","file":"../state.json"}`)
	if invalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid file status=%d", invalid.StatusCode)
	}
	_ = invalid.Body.Close()
}
