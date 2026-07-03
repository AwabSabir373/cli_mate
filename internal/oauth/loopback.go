package oauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LoopbackListener is a single-use loopback HTTP server that captures an OAuth
// redirect on 127.0.0.1 only.
type LoopbackListener struct {
	listener net.Listener
	state    string
	result   chan callbackResult
	server   *http.Server
}

type callbackResult struct {
	code string
	err  error
}

// NewLoopbackListener binds 127.0.0.1:0 (loopback only, OS-assigned port).
func NewLoopbackListener(state string) (*LoopbackListener, error) {
	return NewLoopbackListenerOnPort(state, 0)
}

// NewLoopbackListenerOnPort binds a specific port (0 = OS-assigned).
func NewLoopbackListenerOnPort(state string, port int) (*LoopbackListener, error) {
	if strings.TrimSpace(state) == "" {
		return nil, errors.New("oauth: loopback listener requires a non-empty CSRF state")
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("oauth: start loopback redirect listener: %w", err)
	}
	l := &LoopbackListener{
		listener: ln,
		state:    state,
		result:   make(chan callbackResult, 1),
	}
	l.server = &http.Server{Handler: http.HandlerFunc(l.handle)}
	go func() { _ = l.server.Serve(ln) }()
	return l, nil
}

// RedirectURI returns the http://127.0.0.1:<port>/callback redirect URI.
func (l *LoopbackListener) RedirectURI() string {
	return fmt.Sprintf("http://%s/callback", l.listener.Addr().String())
}

// RedirectURIWithHost returns a redirect URI using the given host and path.
func (l *LoopbackListener) RedirectURIWithHost(host, path string) string {
	_, port, _ := net.SplitHostPort(l.listener.Addr().String())
	return fmt.Sprintf("http://%s:%s%s", host, port, path)
}

func (l *LoopbackListener) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/callback" && r.URL.Path != "/auth/callback" {
		http.NotFound(w, r)
		return
	}
	code, err := parseCallback(r.URL.Query(), l.state)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "Authorization failed. You may close this window.")
	} else {
		_, _ = io.WriteString(w, "Authorization complete. You may close this window.")
	}
	select {
	case l.result <- callbackResult{code: code, err: err}:
	default:
	}
}

// Wait blocks until the callback arrives or ctx is done.
func (l *LoopbackListener) Wait(ctx context.Context) (string, error) {
	select {
	case res := <-l.result:
		return res.code, res.err
	case <-ctx.Done():
		return "", fmt.Errorf("oauth: timed out waiting for authorization callback: %w", ctx.Err())
	}
}

// Close shuts the listener down.
func (l *LoopbackListener) Close() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = l.server.Shutdown(shutdownCtx)
}

// parseCallback validates the redirect query and returns the authorization code.
func parseCallback(values url.Values, wantState string) (string, error) {
	if got := values.Get("state"); got != wantState {
		return "", ErrStateMismatch
	}
	if providerErr := strings.TrimSpace(values.Get("error")); providerErr != "" {
		if desc := strings.TrimSpace(values.Get("error_description")); desc != "" {
			return "", fmt.Errorf("oauth: authorization server returned error %q: %s", providerErr, desc)
		}
		return "", fmt.Errorf("oauth: authorization server returned error %q", providerErr)
	}
	code := strings.TrimSpace(values.Get("code"))
	if code == "" {
		return "", errors.New("oauth: callback missing authorization code")
	}
	return code, nil
}
