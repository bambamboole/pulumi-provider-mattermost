package mattermost

import (
	"errors"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// Client is the small provider-facing wrapper around Mattermost's official v4 client.
type Client struct {
	API *model.Client4
}

// New creates an authenticated Mattermost API client.
func New(baseURL, token string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("mattermost: base URL must not be empty")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("mattermost: token must not be empty")
	}

	api := model.NewAPIv4Client(baseURL)
	api.SetToken(token)
	return &Client{API: api}, nil
}
