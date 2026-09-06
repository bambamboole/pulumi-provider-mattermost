package mattermost

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// ErrMissingToken is returned by every request of a client that was created
// without a token, so a provider configured without credentials fails with a
// clear message at the first API call instead of at configuration time.
var ErrMissingToken = errors.New("mattermost: missing token; set token or MATTERMOST_TOKEN, or obtain one with a Bootstrap resource")

// Client is the small provider-facing wrapper around Mattermost's official v4 client.
type Client struct {
	API     *model.Client4
	BaseURL string
}

// New creates an authenticated Mattermost API client.
func New(baseURL, token string) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("mattermost: token must not be empty")
	}
	client, err := NewAnonymous(baseURL)
	if err != nil {
		return nil, err
	}
	client.API.SetToken(token)
	return client, nil
}

// NewAnonymous creates a client without credentials for the endpoints that
// Mattermost serves unauthenticated: the client configuration, first-user
// signup and login. A successful Login authenticates the client in place.
func NewAnonymous(baseURL string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("mattermost: base URL must not be empty")
	}
	return &Client{API: model.NewAPIv4Client(baseURL), BaseURL: baseURL}, nil
}

// NewMissingToken creates a client whose requests all fail with ErrMissingToken.
func NewMissingToken(baseURL string) *Client {
	api := model.NewAPIv4Client(baseURL)
	api.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, ErrMissingToken
	})}
	return &Client{API: api, BaseURL: baseURL}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
