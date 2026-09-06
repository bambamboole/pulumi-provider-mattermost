package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestIncomingWebhookReadSupportsImport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/hooks/incoming/hook-in" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"hook-in","user_id":"user-1","channel_id":"channel-1","team_id":"team-1","display_name":"Deploy","description":"Deploy notices","username":"deployer","icon_url":"https://example.com/icon.png","channel_locked":true}`))
	}))
	defer server.Close()

	response, err := (IncomingWebhook{}).Read(testContext(t, server.URL), infer.ReadRequest[IncomingWebhookArgs, IncomingWebhookState]{ID: "hook-in"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.ChannelID != "channel-1" || response.State.TeamID != "team-1" || response.State.UserID != "user-1" {
		t.Fatalf("unexpected state: %#v", response.State)
	}
}

func TestOutgoingWebhookReadSupportsImportAndToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/hooks/outgoing/hook-out" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"hook-out","token":"secret-token","creator_id":"user-1","channel_id":"channel-1","team_id":"team-1","trigger_words":["deploy"],"trigger_when":1,"callback_urls":["https://example.com/hook"],"display_name":"Deploy","description":"Deploy trigger","content_type":"application/json","username":"bot","icon_url":"https://example.com/icon.png"}`))
	}))
	defer server.Close()

	response, err := (OutgoingWebhook{}).Read(testContext(t, server.URL), infer.ReadRequest[OutgoingWebhookArgs, OutgoingWebhookState]{ID: "hook-out"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.TeamID != "team-1" || response.Inputs.ChannelID != "channel-1" || response.State.Token != "secret-token" {
		t.Fatalf("unexpected state: %#v", response.State)
	}
	if len(response.Inputs.TriggerWords) != 1 || response.Inputs.TriggerWords[0] != "deploy" {
		t.Fatalf("unexpected triggers: %#v", response.Inputs.TriggerWords)
	}
}
