package tron

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type Option func(*Client)

type Client struct {
	baseURL string
	hc      *http.Client
	headers http.Header

	retryN      int
	retryWait   time.Duration
	maxBodySize int64
	visible     bool
	solid       bool
}

func NewSolid(baseURL string, opts ...Option) *Client {
	opts = append(opts, WithSolid(true))
	return New(baseURL, opts...)
}

func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		hc: &http.Client{
			Timeout: 12 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{Timeout: 6 * time.Second}).DialContext,
			},
		},
		headers:     make(http.Header),
		retryN:      2,
		retryWait:   250 * time.Millisecond,
		maxBodySize: 4 << 20,
		visible:     true,
		solid:       false,
	}

	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

func WithHeader(key, value string) Option {
	return func(c *Client) { c.headers.Set(key, value) }
}

func WithTronGridAPIKey(key string) Option {
	return WithHeader("TRON-PRO-API-KEY", key)
}

func WithSolid(solid bool) Option {
	return func(c *Client) { c.solid = solid }
}

func WithRetry(n int, wait time.Duration) Option {
	return func(c *Client) {
		c.retryN = n
		c.retryWait = wait
	}
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("tron api error: status=%d body=%s", e.StatusCode, e.Body)
}

var canBeUsedSolidMethods = map[string]bool{
	"getnowblock":             true,
	"getblockbynum":           true,
	"getblockbyid":            true,
	"gettransactionbyid":      true,
	"gettransactioninfobyid":  true,
	"getaccount":              true,
	"triggerconstantcontract": true,

	"broadcasttransaction": false,
	"createtransaction":    false,
	"triggersmartcontract": false,
}

func (c *Client) Call(ctx context.Context, methodPath string, req any, out any) error {
	if out == nil {
		return errors.New("out must not be nil")
	}

	canBeUsed, ok := canBeUsedSolidMethods[methodPath]
	if !ok {
		return errors.New("method not found")
	}

	var path string
	if c.solid {
		if !canBeUsed {
			path = "wallet/" + methodPath
		} else {
			path = "walletsolidity/" + methodPath
		}
	} else {
		path = "wallet/" + methodPath
	}

	url := c.baseURL + "/" + path

	var body []byte
	var err error
	if req == nil {
		body = []byte("{}")
	} else {
		body, err = json.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.retryN; attempt++ {
		lastErr = c.doOnce(ctx, url, body, out)
		if lastErr == nil {
			return nil
		}

		if !shouldRetry(lastErr) || attempt == c.retryN {
			return lastErr
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.retryWait):
		}
	}

	return lastErr
}

func (c *Client) doOnce(ctx context.Context, url string, body []byte, out any) error {
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	for k, vv := range c.headers {
		for _, v := range vv {
			r.Header.Add(k, v)
		}
	}

	resp, err := c.hc.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, c.maxBodySize)
	b, err := io.ReadAll(limited)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: string(b)}
	}

	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("unmarshal response: %w; body=%s", err, string(b))
	}
	return nil
}

func shouldRetry(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return false
}
