package httpclient

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"cli_mate/pkg/retry"
)

type Client struct {
	base    *http.Client
	stream  *http.Client
	retries int
}

// New creates an HTTP client.
// timeout controls the dial/TLS/header phase.
// For streaming calls (SSE), a separate client with no body-read deadline is
// used so long-running model responses are never cut off mid-stream.
func New(timeout time.Duration, retries int) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}

	return &Client{
		base: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		// No Timeout on the streaming client — the body is read by the caller
		// via a goroutine until the SSE stream ends or the context is cancelled.
		stream: &http.Client{
			Transport: transport,
		},
		retries: retries,
	}
}

// Stream sends the request and returns the response without a body-read deadline.
// Use this for Server-Sent Events (SSE) / streaming endpoints.
func (c *Client) Stream(ctx context.Context, req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}

	var response *http.Response
	err := retry.Do(ctx, c.retries, 200*time.Millisecond, func() error {
		cloned := req.Clone(ctx)
		if body != nil {
			cloned.Body = io.NopCloser(bytes.NewReader(body))
		}
		var err error
		response, err = c.stream.Do(cloned)
		if err != nil {
			return err
		}
		if response.StatusCode >= http.StatusInternalServerError {
			_ = response.Body.Close()
			return retry.RetriableStatusError{StatusCode: response.StatusCode}
		}
		return nil
	})
	return response, err
}

func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}

	var response *http.Response
	err := retry.Do(ctx, c.retries, 200*time.Millisecond, func() error {
		cloned := req.Clone(ctx)
		if body != nil {
			cloned.Body = io.NopCloser(bytes.NewReader(body))
		}

		var err error
		response, err = c.base.Do(cloned)
		if err != nil {
			return err
		}
		if response.StatusCode >= http.StatusInternalServerError {
			_ = response.Body.Close()
			return retry.RetriableStatusError{StatusCode: response.StatusCode}
		}
		return nil
	})
	return response, err
}
