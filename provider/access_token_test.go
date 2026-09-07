package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestAccessTokenCreateIssuesTokenForBot(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/users/bot-user-1/tokens": `{"id":"token-1","token":"secret-value","user_id":"bot-user-1","description":"support-email","is_active":true}`,
	})

	response, err := (AccessToken{}).Create(testContext(t, server.URL), infer.CreateRequest[AccessTokenArgs]{
		Inputs: AccessTokenArgs{UserID: "bot-user-1", Description: "support-email"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /api/v4/users/bot-user-1/tokens")
	if server.bodies["POST /api/v4/users/bot-user-1/tokens"]["description"] != "support-email" {
		t.Fatalf("unexpected body: %#v", server.bodies["POST /api/v4/users/bot-user-1/tokens"])
	}
	if response.ID != "token-1" || response.Output.Token != "secret-value" || response.Output.UserID != "bot-user-1" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestAccessTokenCreateDryRunDoesNotCallTheAPI(t *testing.T) {
	server := newRecordingServer(t, map[string]string{})

	response, err := (AccessToken{}).Create(testContext(t, server.URL), infer.CreateRequest[AccessTokenArgs]{
		DryRun: true,
		Inputs: AccessTokenArgs{UserID: "bot-user-1", Description: "support-email"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t)
	if response.Output.Token != "" {
		t.Fatal("a preview must not invent a token")
	}
}

func TestAccessTokenReadKeepsTokenAndSyncsDescription(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/users/tokens/token-1": `{"id":"token-1","user_id":"bot-user-1","description":"renamed","is_active":true}`,
	})

	response, err := (AccessToken{}).Read(testContext(t, server.URL), infer.ReadRequest[AccessTokenArgs, AccessTokenState]{
		ID:    "token-1",
		State: AccessTokenState{AccessTokenArgs: AccessTokenArgs{UserID: "bot-user-1", Description: "support-email"}, Token: "secret-value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "token-1" || response.Inputs.Description != "renamed" || response.State.Token != "secret-value" {
		t.Fatalf("unexpected read result: %#v", response)
	}
}

func TestAccessTokenReadTreatsMissingOrDisabledAsGone(t *testing.T) {
	for name, responses := range map[string]map[string]string{
		"disabled": {"GET /api/v4/users/tokens/token-1": `{"id":"token-1","user_id":"bot-user-1","description":"support-email","is_active":false}`},
	} {
		t.Run(name, func(t *testing.T) {
			server := newRecordingServer(t, responses)

			response, err := (AccessToken{}).Read(testContext(t, server.URL), infer.ReadRequest[AccessTokenArgs, AccessTokenState]{ID: "token-1"})
			if err != nil {
				t.Fatal(err)
			}
			if response.ID != "" {
				t.Fatal("expected the token to be reported as gone")
			}
		})
	}

	t.Run("missing", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v4/users/tokens/token-1" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			writeError(w, http.StatusNotFound, "app.user_access_token.get.app_error")
		}))
		defer server.Close()

		response, err := (AccessToken{}).Read(testContext(t, server.URL), infer.ReadRequest[AccessTokenArgs, AccessTokenState]{ID: "token-1"})
		if err != nil {
			t.Fatal(err)
		}
		if response.ID != "" {
			t.Fatal("expected a missing token to be reported as gone")
		}
	})
}

func TestAccessTokenDeleteRevokes(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/users/tokens/revoke": `{"status":"OK"}`,
	})

	_, err := (AccessToken{}).Delete(testContext(t, server.URL), infer.DeleteRequest[AccessTokenState]{ID: "token-1"})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /api/v4/users/tokens/revoke")
	if server.bodies["POST /api/v4/users/tokens/revoke"]["token_id"] != "token-1" {
		t.Fatalf("unexpected body: %#v", server.bodies["POST /api/v4/users/tokens/revoke"])
	}
}
