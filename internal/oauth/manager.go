package oauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrNoToken reports that no token is stored for a key.
var ErrNoToken = errors.New("oauth: no stored token")

const defaultRefreshBuffer = 60 * time.Second
const oidcWellKnownPath = "/.well-known/openid-configuration"
const KeyPrefixProvider = "provider:"
const KeyPrefixMCP = "mcp:"

// Status is a redaction-safe summary of a stored token.
type Status struct {
	Key             string
	HasToken        bool
	HasRefreshToken bool
	TokenType       string
	Account         string
	ExpiresAt       time.Time
	Expired         bool
}

// Manager ties the token store, provider registry, and HTTP client together.
type Manager struct {
	store       *Store
	registry    *Registry
	client      *http.Client
	env         map[string]string
	now         func() time.Time
	buffer      time.Duration
	out         io.Writer
	openBrowser func(authURL string) error
	refreshMu   sync.Mutex
	refreshLocks map[string]*sync.Mutex
}

// ManagerOptions configures a Manager.
type ManagerOptions struct {
	Store        *Store
	Registry     *Registry
	HTTPClient   *http.Client
	Env          map[string]string
	AllowPresets bool
	Now          func() time.Time
	RefreshBuffer time.Duration
	Out          io.Writer
	OpenBrowser  func(authURL string) error
}

// NewManager builds a Manager, filling defaults.
func NewManager(opts ManagerOptions) (*Manager, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("oauth: manager requires a store")
	}
	registry := opts.Registry
	if registry == nil {
		registry = NewRegistry()
	}
	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	buffer := opts.RefreshBuffer
	if buffer <= 0 {
		buffer = defaultRefreshBuffer
	}
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	open := opts.OpenBrowser
	if open == nil {
		open = func(string) error { return nil }
	}
	return &Manager{
		store: opts.Store, registry: registry, client: client,
		env: opts.Env, now: now, buffer: buffer, out: out, openBrowser: open,
	}, nil
}

// LoginOptions configures a single provider login.
type LoginOptions struct {
	Provider   string
	Device     bool
	Timeout    time.Duration
}

// Login runs the provider login, stores the token, and returns a status.
func (m *Manager) Login(ctx context.Context, opts LoginOptions) (Status, error) {
	cfg, flow, err := m.registry.ResolveConfig(opts.Provider, m.env)
	if err != nil {
		return Status{}, err
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	loginCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cfg, err = m.resolveEndpoints(loginCtx, cfg)
	if err != nil {
		return Status{}, err
	}

	useDevice := opts.Device || flow == FlowDevice
	var token Token
	if useDevice {
		token, err = m.loginDevice(loginCtx, cfg)
	} else {
		token, err = m.loginLoopback(loginCtx, cfg)
	}
	if err != nil {
		return Status{}, err
	}

	key := ProviderKey(opts.Provider)
	if err := m.store.Save(key, token); err != nil {
		return Status{}, err
	}
	return m.statusFor(key)
}

// GetFresh returns a valid access token for key, refreshing if needed.
func (m *Manager) GetFresh(ctx context.Context, key string) (string, error) {
	token, err := m.loadToken(key)
	if err != nil {
		return "", err
	}
	if !token.NeedsRefresh(m.now().Unix(), int64(m.buffer.Seconds())) {
		return token.AccessToken, nil
	}
	cfg, err := m.resolveConfigForKey(ctx, key)
	if err != nil {
		return "", err
	}
	return m.refreshAndSave(ctx, key, cfg, token)
}

// Handle401 forces a refresh after an upstream 401.
func (m *Manager) Handle401(ctx context.Context, key string) (string, error) {
	token, err := m.loadToken(key)
	if err != nil {
		return "", err
	}
	cfg, err := m.resolveConfigForKey(ctx, key)
	if err != nil {
		return "", err
	}
	return m.refreshAndSave(ctx, key, cfg, token)
}

// Logout removes a provider's stored token.
func (m *Manager) Logout(name string) (bool, error) {
	return m.store.Delete(ProviderKey(name))
}

// StatusAll returns the status of every provider login.
func (m *Manager) StatusAll() ([]Status, error) {
	return m.store.Status(KeyPrefixProvider)
}

// ProviderKey builds the store key for a provider login.
func ProviderKey(name string) string { return KeyPrefixProvider + name }

func (m *Manager) refreshAndSave(ctx context.Context, key string, cfg Config, current Token) (string, error) {
	lock := m.keyLock(key)
	lock.Lock()
	defer lock.Unlock()

	if reloaded, err := m.loadToken(key); err == nil && reloaded.AccessToken != current.AccessToken {
		return reloaded.AccessToken, nil
	}

	refreshed, err := Refresh(ctx, m.client, cfg, current, func() time.Time { return m.now() })
	if err != nil {
		return "", err
	}
	if err := m.store.Save(key, refreshed); err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}

func (m *Manager) keyLock(key string) *sync.Mutex {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	if m.refreshLocks == nil {
		m.refreshLocks = map[string]*sync.Mutex{}
	}
	lock, ok := m.refreshLocks[key]
	if !ok {
		lock = &sync.Mutex{}
		m.refreshLocks[key] = lock
	}
	return lock
}

func (m *Manager) loadToken(key string) (Token, error) {
	token, ok, err := m.store.Load(key)
	if err != nil {
		return Token{}, err
	}
	if !ok {
		return Token{}, fmt.Errorf("%w for %q", ErrNoToken, key)
	}
	return token, nil
}

func (m *Manager) resolveConfigForKey(ctx context.Context, key string) (Config, error) {
	name := strings.TrimPrefix(key, KeyPrefixProvider)
	if name == key {
		return Config{}, fmt.Errorf("oauth: refresh is only supported for provider tokens (got %q)", key)
	}
	cfg, _, err := m.registry.ResolveConfig(name, m.env)
	if err != nil {
		return Config{}, err
	}
	cfg, err = m.resolveEndpoints(ctx, cfg)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (m *Manager) resolveEndpoints(ctx context.Context, cfg Config) (Config, error) {
	if trimmed(cfg.IssuerURL) == "" {
		return cfg, nil
	}
	if cfg.AuthorizationEndpoint != "" && cfg.TokenEndpoint != "" {
		return cfg, nil
	}
	if meta, err := DiscoverAuthorizationServer(ctx, m.client, cfg.IssuerURL); err == nil {
		if cfg.AuthorizationEndpoint == "" {
			cfg.AuthorizationEndpoint = meta.AuthorizationEndpoint
		}
		if cfg.TokenEndpoint == "" {
			cfg.TokenEndpoint = meta.TokenEndpoint
		}
		if cfg.DeviceAuthorizationEndpoint == "" {
			cfg.DeviceAuthorizationEndpoint = meta.DeviceAuthorizationEndpoint
		}
	}
	return cfg, nil
}

func (m *Manager) loginLoopback(ctx context.Context, cfg Config) (Token, error) {
	if trimmed(cfg.AuthorizationEndpoint) == "" {
		return Token{}, fmt.Errorf("oauth: no authorization endpoint configured")
	}
	state, err := NewState()
	if err != nil {
		return Token{}, err
	}
	pkce, err := NewPKCE()
	if err != nil {
		return Token{}, err
	}
	listener, err := NewLoopbackListener(state)
	if err != nil {
		return Token{}, err
	}
	defer listener.Close()

	redirectURI := listener.RedirectURI()
	authURL, err := BuildAuthorizationURL(cfg, pkce, state, redirectURI, nil)
	if err != nil {
		return Token{}, err
	}

	fmt.Fprintf(m.out, "Open this URL to authorize:\n  %s\n", authURL)
	_ = m.openBrowser(authURL)

	code, err := listener.Wait(ctx)
	if err != nil {
		return Token{}, err
	}
	return ExchangeCode(ctx, m.client, cfg, code, pkce.Verifier, redirectURI, func() time.Time { return m.now() })
}

func (m *Manager) loginDevice(ctx context.Context, cfg Config) (Token, error) {
	auth, err := RequestDeviceCode(ctx, m.client, cfg, func() time.Time { return m.now() })
	if err != nil {
		return Token{}, err
	}
	target := auth.VerificationURIComplete
	if target == "" {
		target = auth.VerificationURI
	}
	fmt.Fprintf(m.out, "To authorize, visit:\n  %s\nand enter code: %s\n", target, auth.UserCode)
	return PollDeviceToken(ctx, m.client, cfg, auth, func() time.Time { return m.now() })
}

func (m *Manager) statusFor(key string) (Status, error) {
	statuses, err := m.store.Status(KeyPrefixProvider)
	if err != nil {
		return Status{}, err
	}
	for _, st := range statuses {
		if st.Key == key {
			return st, nil
		}
	}
	return Status{Key: key}, nil
}
