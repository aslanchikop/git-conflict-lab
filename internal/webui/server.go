// Package webui serves the offline browser interface on localhost.
package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aslanchikop/git-conflict-lab/internal/exercise"
	"github.com/aslanchikop/git-conflict-lab/internal/lab"
)

//go:embed static/*
var assets embed.FS

var safeName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Server struct{ Root string }

type attempt struct {
	Folder string `json:"folder"`
	ID     string `json:"id"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(assets, "static")
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("GET /api/exercises", func(w http.ResponseWriter, r *http.Request) { sendJSON(w, exercise.List()) })
	mux.HandleFunc("GET /api/attempts", func(w http.ResponseWriter, r *http.Request) {
		entries, err := os.ReadDir(s.Root)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out := make([]attempt, 0)
		for _, entry := range entries {
			if !entry.IsDir() || !safeName.MatchString(entry.Name()) {
				continue
			}
			path := filepath.Join(s.Root, entry.Name())
			data, err := os.ReadFile(filepath.Join(path, ".git-conflict-lab", "state.json"))
			if err != nil {
				continue
			}
			var state struct {
				ExerciseID string `json:"exercise_id"`
			}
			if json.Unmarshal(data, &state) != nil {
				continue
			}
			if _, err := exercise.Get(state.ExerciseID); err != nil {
				continue
			}
			status := "In progress"
			if result, err := lab.Check(r.Context(), path); err == nil {
				if result.Passed {
					status = "Completed"
				} else if strings.Contains(result.Message, "not been merged yet") || strings.Contains(result.Message, "has not been applied yet") || strings.HasPrefix(result.Message, "Switch to feature/fast") {
					status = "Not started"
				}
			}
			out = append(out, attempt{Folder: entry.Name(), ID: state.ExerciseID, Path: path, Status: status})
		}
		sendJSON(w, out)
	})
	mux.HandleFunc("POST /api/start", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			http.Error(w, "origin denied", http.StatusForbidden)
			return
		}
		var input struct {
			ID      string `json:"id"`
			Attempt string `json:"attempt"`
		}
		if !decode(w, r, &input) {
			return
		}
		if _, err := exercise.Get(input.ID); err != nil {
			http.Error(w, "unknown exercise", http.StatusBadRequest)
			return
		}
		folder := input.ID
		if input.Attempt != "" {
			if !safeName.MatchString(input.Attempt) {
				http.Error(w, "invalid attempt name", http.StatusBadRequest)
				return
			}
			folder += "-" + input.Attempt
		}
		path, err := lab.GenerateAs(r.Context(), s.Root, input.ID, folder)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		sendJSON(w, attempt{Folder: folder, ID: input.ID, Path: path, Status: "Not started"})
	})
	mux.HandleFunc("POST /api/check", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			http.Error(w, "origin denied", http.StatusForbidden)
			return
		}
		input, ok := requestFolder(w, r, s.Root)
		if !ok {
			return
		}
		result, err := lab.Check(r.Context(), filepath.Join(s.Root, input.Folder))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sendJSON(w, result)
	})
	mux.HandleFunc("POST /api/inspect", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			http.Error(w, "origin denied", http.StatusForbidden)
			return
		}
		input, ok := requestFolder(w, r, s.Root)
		if !ok {
			return
		}
		view, err := lab.InspectFile(r.Context(), filepath.Join(s.Root, input.Folder), input.File)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sendJSON(w, view)
	})
	mux.HandleFunc("POST /api/hint", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			http.Error(w, "origin denied", http.StatusForbidden)
			return
		}
		input, ok := requestFolder(w, r, s.Root)
		if !ok {
			return
		}
		data, err := os.ReadFile(filepath.Join(s.Root, input.Folder, ".git-conflict-lab", "state.json"))
		if err != nil {
			http.Error(w, "exercise not found", http.StatusNotFound)
			return
		}
		var state struct {
			ExerciseID string `json:"exercise_id"`
		}
		if json.Unmarshal(data, &state) != nil {
			http.Error(w, "invalid exercise state", http.StatusBadRequest)
			return
		}
		hint, err := lab.NextHint(filepath.Join(s.Root, input.Folder), state.ExerciseID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sendJSON(w, map[string]string{"hint": hint})
	})
	return mux
}

type folderInput struct {
	Folder string `json:"folder"`
	File   string `json:"file"`
}

func requestFolder(w http.ResponseWriter, r *http.Request, root string) (folderInput, bool) {
	var input folderInput
	if !decode(w, r, &input) {
		return folderInput{}, false
	}
	if !safeName.MatchString(input.Folder) {
		http.Error(w, "invalid folder", http.StatusBadRequest)
		return folderInput{}, false
	}
	info, err := os.Lstat(filepath.Join(root, input.Folder))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		http.Error(w, "exercise folder not found", http.StatusNotFound)
		return folderInput{}, false
	}
	return input, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "JSON required", http.StatusUnsupportedMediaType)
		return false
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(v); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return false
	}
	return true
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || origin == "http://"+r.Host
}

func sendJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// Listen starts a localhost-only server. The returned URL can be opened in a browser.
func (s Server) Listen(ctx context.Context) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	server := &http.Server{Handler: s.Handler()}
	go func() { <-ctx.Done(); _ = server.Close() }()
	go func() { _ = server.Serve(listener) }()
	return fmt.Sprintf("http://%s", listener.Addr().String()), nil
}
