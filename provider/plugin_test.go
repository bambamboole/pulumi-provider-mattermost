package provider

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

const agentsManifest = `{"id":"mattermost-ai","name":"Agents","version":"1.3.0"}`

func TestPluginCreateInstallsFromMarketplaceWritesSettingsAndEnables(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/plugins/marketplace":          agentsManifest,
		"GET /api/v4/config":                        `{"PluginSettings":{"Plugins":{"com.mattermost.calls":{"defaultenabled":true}}}}`,
		"PUT /api/v4/config":                        `{}`,
		"POST /api/v4/plugins/mattermost-ai/enable": `{"status":"OK"}`,
	})
	version := "1.3.0"
	settings := map[string]any{"config": map[string]any{"services": []any{map[string]any{"name": "openai", "apiKey": "sk-1"}}}}

	response, err := (Plugin{}).Create(testContext(t, server.URL), infer.CreateRequest[PluginArgs]{
		Inputs: PluginArgs{PluginID: "mattermost-ai", Version: &version, Settings: settings},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /api/v4/plugins/marketplace", "GET /api/v4/config", "PUT /api/v4/config", "POST /api/v4/plugins/mattermost-ai/enable")
	install := server.bodies["POST /api/v4/plugins/marketplace"]
	if install["id"] != "mattermost-ai" || install["version"] != "1.3.0" {
		t.Fatalf("unexpected install body: %#v", install)
	}
	plugins := server.bodies["PUT /api/v4/config"]["PluginSettings"].(map[string]any)["Plugins"].(map[string]any)
	if !reflect.DeepEqual(plugins["mattermost-ai"], settings) {
		t.Fatalf("unexpected plugin settings: %#v", plugins["mattermost-ai"])
	}
	if !reflect.DeepEqual(plugins["com.mattermost.calls"], map[string]any{"defaultenabled": true}) {
		t.Fatalf("settings of other plugins must be preserved: %#v", plugins)
	}
	if response.ID != "mattermost-ai" || response.Output.Name != "Agents" || response.Output.InstalledVersion != "1.3.0" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestPluginCreateResolvesTheLatestMarketplaceVersion(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/plugins/marketplace":           `[{"manifest":{"id":"other","version":"9.9.9"}},{"manifest":{"id":"mattermost-ai","version":"1.4.0"}}]`,
		"POST /api/v4/plugins/marketplace":          `{"id":"mattermost-ai","name":"Agents","version":"1.4.0"}`,
		"POST /api/v4/plugins/mattermost-ai/enable": `{"status":"OK"}`,
	})

	response, err := (Plugin{}).Create(testContext(t, server.URL), infer.CreateRequest[PluginArgs]{
		Inputs: PluginArgs{PluginID: "mattermost-ai"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "GET /api/v4/plugins/marketplace", "POST /api/v4/plugins/marketplace", "POST /api/v4/plugins/mattermost-ai/enable")
	if server.bodies["POST /api/v4/plugins/marketplace"]["version"] != "1.4.0" {
		t.Fatalf("unexpected install body: %#v", server.bodies["POST /api/v4/plugins/marketplace"])
	}
	if response.Output.Version != nil {
		t.Fatalf("resolving the latest version must not start pinning it: %#v", response.Output)
	}
	if response.Output.InstalledVersion != "1.4.0" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestPluginCreateFailsWhenTheMarketplaceDoesNotOfferThePlugin(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/plugins/marketplace": `[]`,
	})

	_, err := (Plugin{}).Create(testContext(t, server.URL), infer.CreateRequest[PluginArgs]{
		Inputs: PluginArgs{PluginID: "mattermost-ai"},
	})
	if err == nil || !strings.Contains(err.Error(), "not offered by the marketplace") {
		t.Fatalf("expected a marketplace error, got %v", err)
	}
}

func TestPluginCreateInstallsFromURLAndDisables(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/plugins/install_from_url":      agentsManifest,
		"POST /api/v4/plugins/mattermost-ai/disable": `{"status":"OK"}`,
	})
	url := "https://github.com/mattermost/mattermost-plugin-agents/releases/download/v1.3.0/mattermost-ai-1.3.0.tar.gz"
	disabled := false

	response, err := (Plugin{}).Create(testContext(t, server.URL), infer.CreateRequest[PluginArgs]{
		Inputs: PluginArgs{PluginID: "mattermost-ai", DownloadURL: &url, Enabled: &disabled},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /api/v4/plugins/install_from_url", "POST /api/v4/plugins/mattermost-ai/disable")
	if response.Output.InstalledVersion != "1.3.0" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestPluginCreateRejectsABundleOfAnotherPlugin(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/plugins/install_from_url": `{"id":"com.example.other","name":"Other","version":"0.1.0"}`,
	})
	url := "https://example.com/other.tar.gz"

	_, err := (Plugin{}).Create(testContext(t, server.URL), infer.CreateRequest[PluginArgs]{
		Inputs: PluginArgs{PluginID: "mattermost-ai", DownloadURL: &url},
	})
	if err == nil || !strings.Contains(err.Error(), "contains plugin com.example.other") {
		t.Fatalf("expected a mismatch error, got %v", err)
	}
}

func TestPluginCreateDryRunDoesNotCallTheServer(t *testing.T) {
	server := newRecordingServer(t, map[string]string{})

	response, err := (Plugin{}).Create(testContext(t, server.URL), infer.CreateRequest[PluginArgs]{
		DryRun: true,
		Inputs: PluginArgs{PluginID: "mattermost-ai"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t)
	if response.ID != "mattermost-ai" {
		t.Fatalf("unexpected id %q", response.ID)
	}
}

func TestPluginUpdateWithoutSourceChangeOnlyAppliesSettingsAndState(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/config":                         `{"PluginSettings":{"Plugins":{"mattermost-ai":{"config":{"old":true}}}}}`,
		"PUT /api/v4/config":                         `{}`,
		"POST /api/v4/plugins/mattermost-ai/disable": `{"status":"OK"}`,
	})
	version := "1.3.0"
	disabled := false

	response, err := (Plugin{}).Update(testContext(t, server.URL), infer.UpdateRequest[PluginArgs, PluginState]{
		ID:     "mattermost-ai",
		State:  PluginState{PluginArgs: PluginArgs{PluginID: "mattermost-ai", Version: &version}, Name: "Agents", InstalledVersion: "1.3.0"},
		Inputs: PluginArgs{PluginID: "mattermost-ai", Version: &version, Enabled: &disabled, Settings: map[string]any{"config": map[string]any{"new": true}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "GET /api/v4/config", "PUT /api/v4/config", "POST /api/v4/plugins/mattermost-ai/disable")
	plugins := server.bodies["PUT /api/v4/config"]["PluginSettings"].(map[string]any)["Plugins"].(map[string]any)
	if !reflect.DeepEqual(plugins["mattermost-ai"], map[string]any{"config": map[string]any{"new": true}}) {
		t.Fatalf("settings must replace the stored ones: %#v", plugins["mattermost-ai"])
	}
	if response.Output.Name != "Agents" || response.Output.InstalledVersion != "1.3.0" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestPluginUpdateReinstallsWhenTheVersionChanges(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/plugins/marketplace":          `{"id":"mattermost-ai","name":"Agents","version":"1.4.0"}`,
		"POST /api/v4/plugins/mattermost-ai/enable": `{"status":"OK"}`,
	})
	oldVersion, newVersion := "1.3.0", "1.4.0"

	response, err := (Plugin{}).Update(testContext(t, server.URL), infer.UpdateRequest[PluginArgs, PluginState]{
		ID:     "mattermost-ai",
		State:  PluginState{PluginArgs: PluginArgs{PluginID: "mattermost-ai", Version: &oldVersion}, InstalledVersion: "1.3.0"},
		Inputs: PluginArgs{PluginID: "mattermost-ai", Version: &newVersion},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /api/v4/plugins/marketplace", "POST /api/v4/plugins/mattermost-ai/enable")
	if server.bodies["POST /api/v4/plugins/marketplace"]["version"] != "1.4.0" {
		t.Fatalf("unexpected install body: %#v", server.bodies["POST /api/v4/plugins/marketplace"])
	}
	if response.Output.InstalledVersion != "1.4.0" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestPluginReadSupportsImport(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/plugins": `{"active":[],"inactive":[` + agentsManifest + `]}`,
		"GET /api/v4/config":  `{"PluginSettings":{"Plugins":{"mattermost-ai":{"config":{"services":[]}}}}}`,
	})

	response, err := (Plugin{}).Read(testContext(t, server.URL), infer.ReadRequest[PluginArgs, PluginState]{ID: "mattermost-ai"})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "GET /api/v4/plugins", "GET /api/v4/config")
	inputs := response.Inputs
	if inputs.PluginID != "mattermost-ai" || inputs.Version == nil || *inputs.Version != "1.3.0" {
		t.Fatalf("unexpected inputs: %#v", inputs)
	}
	if inputs.Enabled == nil || *inputs.Enabled {
		t.Fatalf("an inactive plugin must import as disabled: %#v", inputs)
	}
	if !reflect.DeepEqual(inputs.Settings, map[string]any{"config": map[string]any{"services": []any{}}}) {
		t.Fatalf("unexpected settings: %#v", inputs.Settings)
	}
	if response.State.Name != "Agents" || response.State.InstalledVersion != "1.3.0" {
		t.Fatalf("unexpected state: %#v", response.State)
	}
}

func TestPluginReadKeepsMaskedSecretsAndReportsVersionDrift(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/plugins": `{"active":[` + agentsManifest + `],"inactive":[]}`,
		"GET /api/v4/config":  `{"PluginSettings":{"Plugins":{"mattermost-ai":{"apikey":"********************************","model":"gpt-5"}}}}`,
	})
	declaredVersion := "1.2.0"
	enabled := true

	response, err := (Plugin{}).Read(testContext(t, server.URL), infer.ReadRequest[PluginArgs, PluginState]{
		ID: "mattermost-ai",
		Inputs: PluginArgs{
			PluginID: "mattermost-ai", Version: &declaredVersion, Enabled: &enabled,
			Settings: map[string]any{"apiKey": "sk-1", "model": "gpt-4"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response.Inputs.Settings, map[string]any{"apikey": "sk-1", "model": "gpt-5"}) {
		t.Fatalf("unexpected settings: %#v", response.Inputs.Settings)
	}
	if response.Inputs.Version == nil || *response.Inputs.Version != "1.3.0" {
		t.Fatalf("refresh must report the installed version: %#v", response.Inputs)
	}
	if response.Inputs.Enabled == nil || !*response.Inputs.Enabled {
		t.Fatalf("unexpected enabled flag: %#v", response.Inputs)
	}
}

func TestPluginReadLeavesUnmanagedSettingsAndVersionAlone(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/plugins": `{"active":[` + agentsManifest + `],"inactive":[]}`,
	})

	response, err := (Plugin{}).Read(testContext(t, server.URL), infer.ReadRequest[PluginArgs, PluginState]{
		ID:     "mattermost-ai",
		Inputs: PluginArgs{PluginID: "mattermost-ai"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "GET /api/v4/plugins")
	if response.Inputs.Settings != nil || response.Inputs.Version != nil {
		t.Fatalf("refresh started managing omitted fields: %#v", response.Inputs)
	}
}

func TestPluginReadReportsRemovedPlugins(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/plugins": `{"active":[],"inactive":[]}`,
	})

	response, err := (Plugin{}).Read(testContext(t, server.URL), infer.ReadRequest[PluginArgs, PluginState]{ID: "mattermost-ai"})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "" {
		t.Fatalf("expected an empty response for a removed plugin, got %#v", response)
	}
}

func TestPluginDeleteIgnoresMissingPlugins(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v4/plugins/mattermost-ai" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"id":"app.plugin.not_installed.app_error","message":"not installed","status_code":404}`))
	}))
	defer server.Close()

	if _, err := (Plugin{}).Delete(testContext(t, server.URL), infer.DeleteRequest[PluginState]{ID: "mattermost-ai"}); err != nil {
		t.Fatal(err)
	}
}
