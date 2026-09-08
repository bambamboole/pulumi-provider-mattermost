package provider

import (
	"reflect"
	"strings"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

const agentsConfigOnServer = `{
  "services": [{"id": "openrouter", "name": "OpenRouter", "type": "openaicompatible", "apiKey": "sk-1", "apiURL": "https://openrouter.ai/api/v1", "orgId": "", "defaultModel": "anthropic/claude-sonnet-5", "tokenLimit": 0, "outputTokenLimit": 0, "streamingTimeoutSeconds": 0, "useResponsesAPI": false}],
  "bots": [{"id": "legacy", "name": "legacy"}],
  "defaultBotName": "",
  "enableTokenUsageLogging": false,
  "enableCallSummary": true,
  "allowedUpstreamHostnames": "",
  "allowUnsafeLinks": false,
  "enableChannelMentionToolCalling": false,
  "allowNativeWebSearchInChannels": false,
  "mcp": {"enabled": true, "servers": [{"name": "docs"}]},
  "webSearch": {"enabled": false}
}`

func openRouterService() AgentsService {
	return AgentsService{
		ID:           "openrouter",
		Name:         "OpenRouter",
		Type:         "openaicompatible",
		APIKey:       "sk-1",
		APIURL:       "https://openrouter.ai/api/v1",
		DefaultModel: "anthropic/claude-sonnet-5",
	}
}

func TestAgentsConfigCreateOverlaysTheServicesOnTheStoredConfiguration(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /plugins/mattermost-ai/admin/config": agentsConfigOnServer,
		"PUT /plugins/mattermost-ai/admin/config": ``,
	})
	enabled := true
	limit := 8192

	service := openRouterService()
	service.OutputTokenLimit = &limit
	response, err := (AgentsConfig{}).Create(testContext(t, server.URL), infer.CreateRequest[AgentsConfigArgs]{
		Inputs: AgentsConfigArgs{Services: []AgentsService{service}, EnableTokenUsageLogging: &enabled},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "GET /plugins/mattermost-ai/admin/config", "PUT /plugins/mattermost-ai/admin/config")
	if response.ID != "mattermost-ai" {
		t.Fatalf("unexpected id %q", response.ID)
	}
	body := server.bodies["PUT /plugins/mattermost-ai/admin/config"]
	services, _ := body["services"].([]any)
	if len(services) != 1 {
		t.Fatalf("unexpected services: %#v", body["services"])
	}
	sent := services[0].(map[string]any)
	if sent["id"] != "openrouter" || sent["type"] != "openaicompatible" || sent["apiKey"] != "sk-1" || sent["apiURL"] != "https://openrouter.ai/api/v1" || sent["outputTokenLimit"] != float64(8192) || sent["tokenLimit"] != float64(0) {
		t.Fatalf("unexpected service body: %#v", sent)
	}
	if body["enableTokenUsageLogging"] != true || body["enableCallSummary"] != true {
		t.Fatalf("declared switches must be applied and undeclared ones kept: %#v", body)
	}
	if mcp, _ := body["mcp"].(map[string]any); mcp == nil || len(mcp["servers"].([]any)) != 1 {
		t.Fatalf("unmanaged parts of the configuration must survive: %#v", body["mcp"])
	}
	if _, kept := body["bots"]; !kept {
		t.Fatal("legacy bots must be left alone")
	}
}

func TestAgentsConfigCreateFailsWhenThePluginIsMissing(t *testing.T) {
	server := newRecordingServer(t, map[string]string{})
	server.Config.Handler = notFoundHandler(t, server)

	_, err := (AgentsConfig{}).Create(testContext(t, server.URL), infer.CreateRequest[AgentsConfigArgs]{
		Inputs: AgentsConfigArgs{Services: []AgentsService{openRouterService()}},
	})
	if err == nil || !strings.Contains(err.Error(), "not installed or not enabled") {
		t.Fatalf("expected a missing-plugin error, got %v", err)
	}
}

func TestAgentsConfigReadRefreshesServicesAndDeclaredSwitches(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /plugins/mattermost-ai/admin/config": agentsConfigOnServer,
	})
	declaredFalse := false
	declared := AgentsConfigArgs{Services: []AgentsService{openRouterService()}, EnableCallSummary: &declaredFalse}

	response, err := (AgentsConfig{}).Read(testContext(t, server.URL), infer.ReadRequest[AgentsConfigArgs, AgentsConfigState]{
		ID: "mattermost-ai", Inputs: declared, State: AgentsConfigState{AgentsConfigArgs: declared},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "mattermost-ai" {
		t.Fatalf("unexpected id %q", response.ID)
	}
	if !reflect.DeepEqual(response.Inputs.Services, []AgentsService{openRouterService()}) {
		t.Fatalf("unexpected services: %#v", response.Inputs.Services)
	}
	if response.Inputs.EnableCallSummary == nil || !*response.Inputs.EnableCallSummary {
		t.Fatalf("drift of a declared switch must be read back: %#v", response.Inputs.EnableCallSummary)
	}
	if response.Inputs.EnableTokenUsageLogging != nil || response.Inputs.AllowUnsafeLinks != nil {
		t.Fatalf("undeclared switches must stay unmanaged: %#v", response.Inputs)
	}
}

func TestAgentsConfigReadKeepsTheDeclaredKeyWhenTheServerHidesIt(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /plugins/mattermost-ai/admin/config": strings.Replace(agentsConfigOnServer, `"apiKey": "sk-1"`, `"apiKey": ""`, 1),
	})
	declared := AgentsConfigArgs{Services: []AgentsService{openRouterService()}}

	response, err := (AgentsConfig{}).Read(testContext(t, server.URL), infer.ReadRequest[AgentsConfigArgs, AgentsConfigState]{
		ID: "mattermost-ai", Inputs: declared, State: AgentsConfigState{AgentsConfigArgs: declared},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.Services[0].APIKey != "sk-1" {
		t.Fatalf("expected the declared key to be kept, got %#v", response.Inputs.Services[0])
	}
}

func TestAgentsConfigReadImportsEverySwitchWithEmptyInputs(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /plugins/mattermost-ai/admin/config": agentsConfigOnServer,
	})

	response, err := (AgentsConfig{}).Read(testContext(t, server.URL), infer.ReadRequest[AgentsConfigArgs, AgentsConfigState]{ID: "mattermost-ai"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.EnableCallSummary == nil || !*response.Inputs.EnableCallSummary || response.Inputs.AllowUnsafeLinks == nil || *response.Inputs.AllowUnsafeLinks {
		t.Fatalf("expected every switch to be imported: %#v", response.Inputs)
	}
	if len(response.Inputs.Services) != 1 {
		t.Fatalf("expected the services to be imported: %#v", response.Inputs.Services)
	}
}

func TestAgentsConfigReadReportsAMissingPluginAsGone(t *testing.T) {
	server := newRecordingServer(t, map[string]string{})
	server.Config.Handler = notFoundHandler(t, server)

	response, err := (AgentsConfig{}).Read(testContext(t, server.URL), infer.ReadRequest[AgentsConfigArgs, AgentsConfigState]{ID: "mattermost-ai"})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "" {
		t.Fatal("a plugin that answers 404 must mark the configuration as gone")
	}
}

func TestAgentsConfigDeleteKeepsTheConfiguration(t *testing.T) {
	server := newRecordingServer(t, map[string]string{})

	if _, err := (AgentsConfig{}).Delete(testContext(t, server.URL), infer.DeleteRequest[AgentsConfigState]{ID: "mattermost-ai"}); err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t)
}
