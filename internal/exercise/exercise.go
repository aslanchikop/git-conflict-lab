// Package exercise provides access to the embedded catalog of practice
// exercise specifications. The catalog ships as JSON files under catalog/
// and is parsed and validated once at package initialization.
package exercise

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"sync"
)

//go:embed catalog/*.json
var catalogFS embed.FS

// Spec describes a single practice exercise.
type Spec struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Difficulty  string   `json:"difficulty"`
	Objective   string   `json:"objective"`
	Description string   `json:"description"`
	Hints       []string `json:"hints"`
}

// ErrNotFound is returned by Get when no exercise matches the given id.
var ErrNotFound = errors.New("exercise not found")

var (
	validID   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	validDiff = map[string]bool{"easy": true, "medium": true, "hard": true}
)

// catalogByIDs parses the embedded catalog at most once. Any failure
// panics: a broken embedded catalog is a programmer error.
var catalogByIDs = sync.OnceValue(func() map[string]Spec {
	byID, err := loadCatalog(catalogFS)
	if err != nil {
		panic(err)
	}
	return byID
})

// init forces catalog parsing (and validation) at package init so that
// invalid catalog data fails fast instead of on first use.
func init() { catalogByIDs() }

// loadCatalog parses and validates every catalog JSON file under dir in
// fsys. It is parametrized over fs.FS so tests can feed synthetic catalogs
// (the production caller passes the embedded catalogFS).
func loadCatalog(fsys fs.FS) (map[string]Spec, error) {
	const dir = "catalog"
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("exercise: read catalog dir: %w", err)
	}
	byID := make(map[string]Spec, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := fs.ReadFile(fsys, dir+"/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("exercise: read %s: %w", entry.Name(), err)
		}
		var spec Spec
		if err := json.Unmarshal(raw, &spec); err != nil {
			return nil, fmt.Errorf("exercise: parse %s: %w", entry.Name(), err)
		}
		if err := spec.Validate(); err != nil {
			return nil, fmt.Errorf("exercise: %s: %w", entry.Name(), err)
		}
		if _, dup := byID[spec.ID]; dup {
			return nil, fmt.Errorf("exercise: duplicate exercise id %q", spec.ID)
		}
		byID[spec.ID] = spec
	}
	if len(byID) == 0 {
		return nil, errors.New("exercise: catalog is empty")
	}
	return byID, nil
}

// Validate reports whether the spec satisfies all catalog invariants:
// required fields present, a known difficulty, a kebab-case id, and at
// least one hint.
func (s Spec) Validate() error {
	switch {
	case strings.TrimSpace(s.ID) == "":
		return errors.New("spec id is empty")
	case strings.TrimSpace(s.Title) == "":
		return errors.New("spec title is empty")
	case strings.TrimSpace(s.Difficulty) == "":
		return errors.New("spec difficulty is empty")
	case strings.TrimSpace(s.Objective) == "":
		return errors.New("spec objective is empty")
	case !validDiff[s.Difficulty]:
		return fmt.Errorf("spec %q: unknown difficulty %q", s.ID, s.Difficulty)
	case !validID.MatchString(s.ID):
		return fmt.Errorf("spec id %q: must be kebab-case (lowercase letters, digits, hyphens)", s.ID)
	case len(s.Hints) < 1:
		return fmt.Errorf("spec %q: at least one hint is required", s.ID)
	}
	return nil
}

// catalog returns the parsed catalog. Parsing happens once at package
// init; invalid data panics there.
func catalog() map[string]Spec {
	return catalogByIDs()
}

// List returns every exercise in the catalog sorted by ID. The returned
// slice is a defensive copy; mutating it does not affect the catalog.
func List() []Spec {
	byID := catalog()
	specs := make([]Spec, 0, len(byID))
	for _, spec := range byID {
		specs = append(specs, copySpec(spec))
	}
	sortSpecs(specs)
	return specs
}

// Get returns the exercise with the given id. The id is trimmed of
// surrounding whitespace; the empty id is never found.
func Get(id string) (Spec, error) {
	id = strings.TrimSpace(id)
	byID := catalog()
	spec, ok := byID[id]
	if !ok {
		return Spec{}, fmt.Errorf("exercise %q: %w", id, ErrNotFound)
	}
	return copySpec(spec), nil
}

func copySpec(s Spec) Spec {
	out := s
	out.Hints = append([]string(nil), s.Hints...)
	return out
}

func sortSpecs(specs []Spec) {
	// Small insertion sort keeps the package free of sort import churn;
	// deterministic by ID, tie-break on title for stable output.
	for i := 1; i < len(specs); i++ {
		for j := i; j > 0 && less(specs[j], specs[j-1]); j-- {
			specs[j], specs[j-1] = specs[j-1], specs[j]
		}
	}
}

func less(a, b Spec) bool {
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	return a.Title < b.Title
}
