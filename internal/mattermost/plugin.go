package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PluginError is the failed response of a plugin endpoint.
type PluginError struct {
	Status  int
	Message string
}

func (e *PluginError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("mattermost: plugin endpoint responded with status %d", e.Status)
	}
	return fmt.Sprintf("mattermost: plugin endpoint responded with status %d: %s", e.Status, e.Message)
}

// IsPluginNotFound reports whether err is a plugin endpoint's 404, which
// plugins return both for an unknown object and, through Mattermost, for a
// plugin that is not installed or not enabled.
func IsPluginNotFound(err error) bool {
	var pluginErr *PluginError
	return errors.As(err, &pluginErr) && pluginErr.Status == http.StatusNotFound
}

// PluginRequest calls an endpoint a plugin serves under /plugins/<pluginID>
// with the client's token. Mattermost hands the request to the plugin with the
// caller's user ID, so the token's user needs whatever the endpoint requires.
// A JSON body is sent when body is not nil and the JSON response is decoded
// into out when out is not nil.
func (c *Client) PluginRequest(ctx context.Context, method, pluginID, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("mattermost: encoding the request to plugin %s: %w", pluginID, err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.BaseURL+"/plugins/"+pluginID+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	// Plugin endpoints are protected against CSRF like the REST API.
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.API.AuthToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.API.AuthToken)
	}
	httpClient := c.API.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode >= http.StatusBadRequest {
		return &PluginError{Status: response.StatusCode, Message: pluginErrorMessage(data)}
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("mattermost: decoding the response of plugin %s: %w", pluginID, err)
		}
	}
	return nil
}

// pluginErrorMessage extracts the message of a JSON error body such as
// {"error": "..."} or {"message": "..."} and falls back to the raw body.
func pluginErrorMessage(data []byte) string {
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &body); err == nil {
		if body.Error != "" {
			return body.Error
		}
		if body.Message != "" {
			return body.Message
		}
	}
	return strings.TrimSpace(string(data))
}
