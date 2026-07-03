package specialist

import (
	"os"
	"path/filepath"
	"strings"
)

// Registry manages available specialist manifests.
type Registry struct {
	manifests map[string]*Manifest
}

// NewRegistry creates a registry and loads built-in specialists.
func NewRegistry() *Registry {
	r := &Registry{
		manifests: make(map[string]*Manifest),
	}
	for _, m := range Builtins() {
		r.manifests[m.Metadata.Name] = &m
	}
	return r
}

// LoadFromDir loads specialist manifests from a directory of markdown files.
// Each file should have YAML-like frontmatter and a markdown body.
func (r *Registry) LoadFromDir(dir string, location Location) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		name = strings.ToLower(name)
		if name == "" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		m, err := parseManifestFile(path, name, location)
		if err != nil {
			continue
		}

		r.manifests[name] = m
	}
}

// Get returns a specialist manifest by name, or nil if not found.
func (r *Registry) Get(name string) *Manifest {
	return r.manifests[name]
}

// List returns all available specialist names.
func (r *Registry) List() []string {
	var names []string
	for name := range r.manifests {
		names = append(names, name)
	}
	return names
}

// All returns all available manifests.
func (r *Registry) All() []*Manifest {
	var manifests []*Manifest
	for _, m := range r.manifests {
		manifests = append(manifests, m)
	}
	return manifests
}

func parseManifestFile(path, name string, location Location) (*Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var (
		description string
		tools       string
		body        strings.Builder
		inFrontmatter bool
		frontmatterDone bool
	)

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if !frontmatterDone {
			if strings.TrimSpace(line) == "---" {
				if !inFrontmatter {
					inFrontmatter = true
					continue
				}
				frontmatterDone = true
				continue
			}
			if inFrontmatter {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					switch strings.ToLower(key) {
					case "description":
						description = value
					case "tools":
						tools = value
					}
				}
				continue
			}
		}

		body.WriteString(line)
		body.WriteString("\n")
	}

	m := &Manifest{
		Metadata: Metadata{
			Name:        name,
			Description: description,
			Tools:       parseToolList(tools),
		},
		SystemPrompt: strings.TrimSpace(body.String()),
		Location:     location,
		FilePath:     path,
	}

	if err := Validate(m); err != nil {
		return nil, err
	}
	return m, nil
}

func parseToolList(tools string) []string {
	if tools == "" {
		return []string{"read-only"}
	}
	var result []string
	for _, t := range strings.Split(tools, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}
