package provider

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

const (
	envBaseURL = "MATTERMOST_BASE_URL"
	envToken   = "MATTERMOST_TOKEN"
)

// Config holds provider-level configuration.
type Config struct {
	BaseURL string `pulumi:"baseUrl,optional"`
	Token   string `pulumi:"token,optional" provider:"secret"`

	client *mm.Client
}

func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c, "Manage resources on a Mattermost instance through the Mattermost API v4.")
	a.Describe(&c.BaseURL, "Base URL of the Mattermost instance, e.g. https://mattermost.example.com. Defaults to MATTERMOST_BASE_URL.")
	a.Describe(&c.Token, "Mattermost personal access token or bot token. Defaults to MATTERMOST_TOKEN.")
	a.SetDefault(&c.BaseURL, "", envBaseURL)
	a.SetDefault(&c.Token, "", envToken)
}

func (c *Config) Configure(_ context.Context) error {
	baseURL := firstNonEmpty(c.BaseURL, os.Getenv(envBaseURL))
	token := firstNonEmpty(c.Token, os.Getenv(envToken))
	if strings.TrimSpace(baseURL) == "" {
		return errors.New("mattermost: missing base URL; set baseUrl or MATTERMOST_BASE_URL")
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("mattermost: missing token; set token or MATTERMOST_TOKEN")
	}
	client, err := mm.New(baseURL, token)
	if err != nil {
		return err
	}
	c.client = client
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
