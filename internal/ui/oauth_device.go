package ui

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"cli_mate/internal/oauth"
)

func oauthPreferDeviceFlow() bool {
	if strings.TrimSpace(os.Getenv("CLI_MATE_OAUTH_DEVICE")) != "" {
		return true
	}
	if strings.TrimSpace(os.Getenv("SSH_CONNECTION")) != "" || strings.TrimSpace(os.Getenv("SSH_TTY")) != "" {
		return true
	}
	if runtime.GOOS == "linux" &&
		strings.TrimSpace(os.Getenv("DISPLAY")) == "" &&
		strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) == "" {
		return true
	}
	return false
}

func oauthDeviceLogin(name string) (oauth.Status, error) {
	store, err := oauth.NewStore(oauth.StoreOptions{})
	if err != nil {
		return oauth.Status{}, err
	}
	manager, err := oauth.NewManager(oauth.ManagerOptions{
		Store:        store,
		HTTPClient:   &http.Client{Timeout: 60 * time.Second},
		OpenBrowser:  func(string) error { return nil },
		AllowPresets: true,
	})
	if err != nil {
		return oauth.Status{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return manager.Login(ctx, oauth.LoginOptions{Provider: name, Device: true})
}

func oauthStoredToken(ctx context.Context, providerID string) string {
	store, err := oauth.NewStore(oauth.StoreOptions{})
	if err != nil {
		return ""
	}
	manager, err := oauth.NewManager(oauth.ManagerOptions{
		Store:        store,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		AllowPresets: true,
	})
	if err != nil {
		return ""
	}
	token, err := manager.GetFresh(ctx, oauth.ProviderKey(providerID))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(token)
}
