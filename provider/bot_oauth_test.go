package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestBotReadSupportsImport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/bots/bot-user-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"bot-user-1","username":"deploy-bot","display_name":"Deploy Bot","description":"Deploys things","owner_id":"owner-1"}`))
	}))
	defer server.Close()

	response, err := (Bot{}).Read(testContext(t, server.URL), infer.ReadRequest[BotArgs, BotState]{ID: "bot-user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.Username != "deploy-bot" || response.State.OwnerID != "owner-1" {
		t.Fatalf("unexpected bot state: %#v", response.State)
	}
}

func TestOAuthAppReadPreservesClientSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/oauth/apps/oauth-app-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"oauth-app-1","creator_id":"owner-1","name":"CI","description":"CI integration","icon_url":"https://example.com/icon.png","callback_urls":["https://example.com/callback"],"homepage":"https://example.com","is_trusted":true}`))
	}))
	defer server.Close()

	response, err := (OAuthApp{}).Read(testContext(t, server.URL), infer.ReadRequest[OAuthAppArgs, OAuthAppState]{
		ID: "oauth-app-1",
		State: OAuthAppState{ClientSecret: "existing-secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.State.ClientSecret != "existing-secret" {
		t.Fatal("refresh discarded the OAuth client secret")
	}
	if response.Inputs.Name != "CI" || len(response.Inputs.CallbackURLs) != 1 {
		t.Fatalf("unexpected OAuth app inputs: %#v", response.Inputs)
	}
}
