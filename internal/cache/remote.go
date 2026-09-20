package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrRemoteNotFound = errors.New("remote cache entry not found")
)

// RemoteBackend defines the interface for distributed / remote build caching.
type RemoteBackend interface {
	Has(ctx context.Context, hash string) (bool, error)
	Get(ctx context.Context, hash string) (*Entry, []byte, map[string][]byte, error)
	Put(ctx context.Context, hash string, entry *Entry, logBytes []byte, artifacts map[string][]byte) error
}

// RemoteBundle packages metadata, logs, and artifacts for single-roundtrip remote caching.
type RemoteBundle struct {
	Entry     Entry             `json:"entry"`
	OutputLog []byte            `json:"output_log"`
	Artifacts map[string][]byte `json:"artifacts"`
}

// HTTPRemoteBackend implements RemoteBackend over standard HTTP/REST.
type HTTPRemoteBackend struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

// NewHTTPRemoteBackend creates a new HTTP-based remote cache client.
func NewHTTPRemoteBackend(baseURL, token string, timeout time.Duration) *HTTPRemoteBackend {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &HTTPRemoteBackend{
		BaseURL: baseURL,
		Token:   token,
		Client:  &http.Client{Timeout: timeout},
	}
}

func (h *HTTPRemoteBackend) Has(ctx context.Context, hash string) (bool, error) {
	url := fmt.Sprintf("%s/%s", h.BaseURL, hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("unexpected status code %d from remote cache", resp.StatusCode)
}

func (h *HTTPRemoteBackend) Get(ctx context.Context, hash string) (*Entry, []byte, map[string][]byte, error) {
	url := fmt.Sprintf("%s/%s", h.BaseURL, hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, nil, ErrRemoteNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, fmt.Errorf("remote cache error status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reading remote cache payload: %w", err)
	}

	var bundle RemoteBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, nil, nil, fmt.Errorf("decoding remote cache bundle: %w", err)
	}

	return &bundle.Entry, bundle.OutputLog, bundle.Artifacts, nil
}

func (h *HTTPRemoteBackend) Put(ctx context.Context, hash string, entry *Entry, logBytes []byte, artifacts map[string][]byte) error {
	bundle := RemoteBundle{
		Entry:     *entry,
		OutputLog: logBytes,
		Artifacts: artifacts,
	}

	data, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("encoding remote cache bundle: %w", err)
	}

	url := fmt.Sprintf("%s/%s", h.BaseURL, hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("remote cache put returned status %d", resp.StatusCode)
	}

	return nil
}
