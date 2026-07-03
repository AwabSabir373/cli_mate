package oauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const storeSchemaVersion = 1

var keyPattern = regexp.MustCompile(`^(provider|mcp):[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// ValidateKey reports whether key is a well-formed namespaced token key.
func ValidateKey(key string) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("oauth: invalid token key %q (want \"provider:<name>\" or \"mcp:<name>\")", key)
	}
	return nil
}

type storeFile struct {
	SchemaVersion int              `json:"schemaVersion"`
	Tokens        map[string]Token `json:"tokens"`
}

// Store persists OAuth tokens as a JSON blob.
type Store struct {
	path string
	mu   sync.Mutex
	now  func() time.Time
}

// StoreOptions configures the token store.
type StoreOptions struct {
	FilePath string
	Env      map[string]string
	Now      func() time.Time
}

// NewStore builds a token store.
func NewStore(opts StoreOptions) (*Store, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	path := opts.FilePath
	if path == "" {
		var err error
		path, err = resolveStorePath(opts.Env)
		if err != nil {
			return nil, err
		}
	}
	return &Store{path: path, now: now}, nil
}

// resolveStorePath determines the on-disk location for OAuth tokens.
func resolveStorePath(env map[string]string) (string, error) {
	if override := strings.TrimSpace(envValue(env, "CLI_MATE_OAUTH_TOKENS_PATH")); override != "" {
		if filepath.IsAbs(override) {
			return filepath.Clean(override), nil
		}
		return filepath.Abs(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("oauth: resolve user home: %w", err)
	}
	return filepath.Join(home, ".config", "cli_mate", "oauth-tokens.json"), nil
}

// Save persists a token under key.
func (s *Store) Save(key string, token Token) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeState(func(state *storeFile) {
		state.Tokens[key] = token
	})
}

// Load returns the token for key.
func (s *Store) Load(key string) (Token, bool, error) {
	if err := ValidateKey(key); err != nil {
		return Token{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.readState()
	if err != nil {
		return Token{}, false, err
	}
	token, ok := state.Tokens[key]
	return token, ok, nil
}

// Delete removes the token for key.
func (s *Store) Delete(key string) (bool, error) {
	if err := ValidateKey(key); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var removed bool
	err := s.writeState(func(state *storeFile) {
		if _, ok := state.Tokens[key]; !ok {
			return
		}
		delete(state.Tokens, key)
		removed = true
	})
	return removed, err
}

// Status returns redaction-safe summaries of stored tokens.
func (s *Store) Status(prefix string) ([]Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.readState()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(state.Tokens))
	for k := range state.Tokens {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	now := s.now()
	out := make([]Status, 0, len(keys))
	for _, k := range keys {
		token := state.Tokens[k]
		expiresAt := time.Unix(token.ExpiresAt, 0)
		out = append(out, Status{
			Key:             k,
			HasToken:        strings.TrimSpace(token.AccessToken) != "",
			HasRefreshToken: strings.TrimSpace(token.RefreshToken) != "",
			Account:         token.Account,
			ExpiresAt:       expiresAt,
			Expired:         token.Expired(now.Unix()),
		})
	}
	return out, nil
}

func (s *Store) readState() (storeFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return emptyStoreFile(), nil
		}
		return storeFile{}, err
	}

	var state storeFile
	if err := json.Unmarshal(data, &state); err != nil {
		return storeFile{}, fmt.Errorf("oauth: invalid token store at %s: %w", s.path, err)
	}
	if state.SchemaVersion != storeSchemaVersion {
		return storeFile{}, fmt.Errorf("oauth: invalid token store at %s: unsupported schemaVersion", s.path)
	}
	if state.Tokens == nil {
		state.Tokens = map[string]Token{}
	}
	return state, nil
}

func (s *Store) writeState(update func(*storeFile)) error {
	state, err := s.readState()
	if err != nil {
		return err
	}

	update(&state)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

func emptyStoreFile() storeFile {
	return storeFile{SchemaVersion: storeSchemaVersion, Tokens: map[string]Token{}}
}

// envValue reads a variable from a map or environment.
func envValue(env map[string]string, key string) string {
	if env != nil {
		return env[key]
	}
	return os.Getenv(key)
}

// FilePath returns the token store file path.
func (s *Store) FilePath() string { return s.path }

// FormatStatuses renders human-readable status without leaking token material.
func FormatStatuses(statuses []Status) string {
	if len(statuses) == 0 {
		return "No OAuth provider logins are stored."
	}
	var b strings.Builder
	for i, st := range statuses {
		if i > 0 {
			b.WriteByte('\n')
		}
		name := strings.TrimPrefix(st.Key, KeyPrefixProvider)
		b.WriteString(name)
		b.WriteString(": ")
		if !st.HasToken {
			b.WriteString("no token")
			continue
		}
		b.WriteString("logged in")
		if st.Account != "" {
			b.WriteString(" as " + st.Account)
		}
		if st.HasRefreshToken {
			b.WriteString(" (refreshable)")
		}
		if !st.ExpiresAt.IsZero() {
			if st.Expired {
				b.WriteString(", expired at ")
			} else {
				b.WriteString(", expires ")
			}
			b.WriteString(st.ExpiresAt.UTC().Format(time.RFC3339))
		}
	}
	return b.String()
}
