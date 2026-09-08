package provider

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

const agentOnServer = `{
  "id": "agent-1", "name": "ai", "displayName": "AI", "serviceID": "openrouter", "model": "", "customInstructions": "Be brief.",
  "channelAccessLevel": 2, "channelIDs": ["chan-1"], "userAccessLevel": 0, "userIDs": [], "teamIDs": null, "adminUserIDs": [],
  "enableVision": true, "disableTools": false, "enabledNativeTools": ["search"], "autoEnableNewMCPTools": false,
  "enabledMCPTools": [{"server_origin": "docs", "tool_name": "lookup"}], "mcpDynamicToolLoading": true,
  "reasoningEnabled": false, "reasoningEffort": "", "thinkingBudget": 0, "structuredOutputEnabled": false, "maxToolTurns": 0,
  "botUserID": "bot-user-1", "creatorID": "user-1"
}`

// notFoundHandler answers every request the way Mattermost answers for a
// plugin that is not running, after recording the call.
func notFoundHandler(t *testing.T, server *recordingServer) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.calls = append(server.calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "plugin not found"}`))
	})
}

func accessLevel(level AgentAccessLevel) *AgentAccessLevel {
	return &level
}

func agentArgs() AgentArgs {
	return AgentArgs{
		Username:           "ai",
		DisplayName:        "AI",
		ServiceID:          "openrouter",
		CustomInstructions: "Be brief.",
		ChannelAccessLevel: accessLevel(AgentAccessBlock),
		ChannelIDs:         []string{"chan-1"},
		UserAccessLevel:    accessLevel(AgentAccessAll),
		EnableVision:       true,
		EnabledNativeTools: []string{"search"},
	}
}

func TestAgentCheckAppliesDefaultsAndDropsEmptyLists(t *testing.T) {
	inputs := mustInputs(t, map[string]string{"username": "ai", "displayName": "AI", "serviceId": "openrouter"})
	response, err := (Agent{}).Check(testContext(t, "https://chat.example.com"), infer.CheckRequest{NewInputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Failures) != 0 {
		t.Fatalf("unexpected failures %#v", response.Failures)
	}
	if *response.Inputs.ChannelAccessLevel != AgentAccessAll || *response.Inputs.UserAccessLevel != AgentAccessAll || response.Inputs.ChannelIDs != nil {
		t.Fatalf("defaults not applied: %#v", response.Inputs)
	}
}

func TestAgentCreatePostsTheAgentAndKeepsItsIDs(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /plugins/mattermost-ai/agents": agentOnServer,
	})

	response, err := (Agent{}).Create(testContext(t, server.URL), infer.CreateRequest[AgentArgs]{Inputs: agentArgs()})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /plugins/mattermost-ai/agents")
	body := server.bodies["POST /plugins/mattermost-ai/agents"]
	if body["username"] != "ai" || body["displayName"] != "AI" || body["serviceID"] != "openrouter" || body["channelAccessLevel"] != float64(2) || body["userAccessLevel"] != float64(0) {
		t.Fatalf("unexpected body: %#v", body)
	}
	if !reflect.DeepEqual(body["channelIDs"], []any{"chan-1"}) || !reflect.DeepEqual(body["userIDs"], []any{}) || !reflect.DeepEqual(body["enabledMCPTools"], []any{}) {
		t.Fatalf("lists must be sent as arrays: %#v", body)
	}
	if response.ID != "agent-1" || response.Output.BotUserID != "bot-user-1" || response.Output.Username != "ai" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestAgentCreateSurfacesThePluginError(t *testing.T) {
	server := newRecordingServer(t, map[string]string{})
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error": "creating more than 1 self-service agent(s) requires an E20 or Enterprise license"}`))
	})

	_, err := (Agent{}).Create(testContext(t, server.URL), infer.CreateRequest[AgentArgs]{Inputs: agentArgs()})
	if err == nil || !strings.Contains(err.Error(), "requires an E20 or Enterprise license") {
		t.Fatalf("expected the plugin's message, got %v", err)
	}
}

func TestAgentReadMapsTheAgentBackToInputs(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /plugins/mattermost-ai/agents/agent-1": agentOnServer,
	})

	response, err := (Agent{}).Read(testContext(t, server.URL), infer.ReadRequest[AgentArgs, AgentState]{ID: "agent-1", Inputs: agentArgs()})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "agent-1" || !reflect.DeepEqual(response.Inputs, agentArgs()) {
		t.Fatalf("unexpected inputs: %#v", response.Inputs)
	}
	if response.State.BotUserID != "bot-user-1" {
		t.Fatalf("unexpected state: %#v", response.State)
	}
}

func TestAgentReadReportsAMissingAgentAsGone(t *testing.T) {
	server := newRecordingServer(t, map[string]string{})
	server.Config.Handler = notFoundHandler(t, server)

	response, err := (Agent{}).Read(testContext(t, server.URL), infer.ReadRequest[AgentArgs, AgentState]{ID: "agent-1", Inputs: agentArgs()})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "" {
		t.Fatal("a 404 must mark the agent as gone")
	}
}

func TestAgentUpdateReplacesTheAgentAndCarriesMCPToolsOver(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /plugins/mattermost-ai/agents/agent-1": agentOnServer,
		"PUT /plugins/mattermost-ai/agents/agent-1": strings.Replace(agentOnServer, `"displayName": "AI"`, `"displayName": "Assistant"`, 1),
	})
	inputs := agentArgs()
	inputs.DisplayName = "Assistant"
	inputs.ChannelAccessLevel = accessLevel(AgentAccessAll)
	inputs.ChannelIDs = nil

	response, err := (Agent{}).Update(testContext(t, server.URL), infer.UpdateRequest[AgentArgs, AgentState]{
		ID: "agent-1", Inputs: inputs, State: AgentState{AgentArgs: agentArgs(), BotUserID: "bot-user-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "GET /plugins/mattermost-ai/agents/agent-1", "PUT /plugins/mattermost-ai/agents/agent-1")
	body := server.bodies["PUT /plugins/mattermost-ai/agents/agent-1"]
	if _, sent := body["username"]; sent {
		t.Fatal("the username cannot change and must stay out of the update")
	}
	if body["displayName"] != "Assistant" || body["channelAccessLevel"] != float64(0) || !reflect.DeepEqual(body["channelIDs"], []any{}) {
		t.Fatalf("unexpected body: %#v", body)
	}
	if body["mcpDynamicToolLoading"] != true || len(body["enabledMCPTools"].([]any)) != 1 {
		t.Fatalf("the MCP tool selection must be carried over: %#v", body)
	}
	if response.Output.DisplayName != "Assistant" || response.Output.BotUserID != "bot-user-1" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestAgentDeleteToleratesAMissingAgent(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"DELETE /plugins/mattermost-ai/agents/agent-1": ``,
	})
	if _, err := (Agent{}).Delete(testContext(t, server.URL), infer.DeleteRequest[AgentState]{ID: "agent-1"}); err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "DELETE /plugins/mattermost-ai/agents/agent-1")

	gone := newRecordingServer(t, map[string]string{})
	gone.Config.Handler = notFoundHandler(t, gone)
	if _, err := (Agent{}).Delete(testContext(t, gone.URL), infer.DeleteRequest[AgentState]{ID: "agent-1"}); err != nil {
		t.Fatalf("a missing agent must not fail the delete, got %v", err)
	}
}
