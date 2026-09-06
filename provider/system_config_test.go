package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestSystemConfigReadOnlyReturnsManagedFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/config" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServiceSettings":{"SiteURL":"https://chat.example.com","EnableIncomingWebhooks":true,"EnableOutgoingWebhooks":false,"EnableBotAccountCreation":true}}`))
	}))
	defer server.Close()

	ctx := testContext(t, server.URL)
	declared := true
	response, err := (SystemConfig{}).Read(ctx, infer.ReadRequest[SystemConfigArgs, SystemConfigState]{
		ID:     "system",
		Inputs: SystemConfigArgs{EnableIncomingWebhooks: &declared},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "system" {
		t.Fatalf("unexpected id %q", response.ID)
	}
	if response.Inputs.EnableIncomingWebhooks == nil || !*response.Inputs.EnableIncomingWebhooks {
		t.Fatal("managed setting was not refreshed")
	}
	if response.Inputs.SiteURL != nil || response.Inputs.EnableOutgoingWebhooks != nil || response.Inputs.EnableBotAccountCreation != nil {
		t.Fatalf("refresh started managing omitted fields: %#v", response.Inputs)
	}
}

func TestSystemConfigUpdatePreservesUnmanagedSettings(t *testing.T) {
	var updated map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/config" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ServiceSettings":{"SiteURL":"https://chat.example.com","EnableIncomingWebhooks":false,"EnableOutgoingWebhooks":true},"TeamSettings":{"SiteName":"Keep Me"}}`))
		case http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				t.Fatal(err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(updated)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	ctx := testContext(t, server.URL)
	enabled := true
	_, err := (SystemConfig{}).Update(ctx, infer.UpdateRequest[SystemConfigArgs, SystemConfigState]{
		ID:     "system",
		Inputs: SystemConfigArgs{EnableIncomingWebhooks: &enabled},
	})
	if err != nil {
		t.Fatal(err)
	}

	service, ok := updated["ServiceSettings"].(map[string]any)
	if !ok {
		t.Fatalf("missing ServiceSettings in update: %#v", updated)
	}
	if service["EnableIncomingWebhooks"] != true {
		t.Fatalf("managed field not updated: %#v", service)
	}
	if service["EnableOutgoingWebhooks"] != true {
		t.Fatalf("unmanaged service setting was not preserved: %#v", service)
	}
	team, ok := updated["TeamSettings"].(map[string]any)
	if !ok || team["SiteName"] != "Keep Me" {
		t.Fatalf("unmanaged config section was not preserved: %#v", updated)
	}
}

func TestSystemConfigImportAdoptsExposedSettings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServiceSettings":{"SiteURL":"https://chat.example.com","EnableCommands":true,"EnableOAuthServiceProvider":true}}`))
	}))
	defer server.Close()

	ctx := testContext(t, server.URL)
	response, err := (SystemConfig{}).Read(ctx, infer.ReadRequest[SystemConfigArgs, SystemConfigState]{ID: "system"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.SiteURL == nil || *response.Inputs.SiteURL != "https://chat.example.com" {
		t.Fatalf("import did not adopt siteUrl: %#v", response.Inputs)
	}
	if response.Inputs.EnableCommands == nil || !*response.Inputs.EnableCommands {
		t.Fatalf("import did not adopt enableCommands: %#v", response.Inputs)
	}
}
