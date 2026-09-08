package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

const agentsPath = "/agents"

// AgentAccessLevel decides which channels or users may use an agent.
type AgentAccessLevel string

const (
	AgentAccessAll   AgentAccessLevel = "all"
	AgentAccessAllow AgentAccessLevel = "allow"
	AgentAccessBlock AgentAccessLevel = "block"
	AgentAccessNone  AgentAccessLevel = "none"
)

func (AgentAccessLevel) Values() []infer.EnumValue[AgentAccessLevel] {
	return []infer.EnumValue[AgentAccessLevel]{
		{Name: "All", Value: AgentAccessAll, Description: "Every channel or user."},
		{Name: "Allow", Value: AgentAccessAllow, Description: "Only the listed channels or users."},
		{Name: "Block", Value: AgentAccessBlock, Description: "Every channel or user except the listed ones."},
		{Name: "None", Value: AgentAccessNone, Description: "No channel or user."},
	}
}

// The plugin stores access levels as the index of this order.
var agentAccessLevels = []AgentAccessLevel{AgentAccessAll, AgentAccessAllow, AgentAccessBlock, AgentAccessNone}

func agentAccessLevelIndex(level *AgentAccessLevel) int {
	if level == nil {
		return 0
	}
	for index, candidate := range agentAccessLevels {
		if candidate == *level {
			return index
		}
	}
	return 0
}

func agentAccessLevelFromIndex(index int) *AgentAccessLevel {
	level := AgentAccessAll
	if index >= 0 && index < len(agentAccessLevels) {
		level = agentAccessLevels[index]
	}
	return &level
}

// Agent is a self-service agent of the Agents plugin (mattermost-ai): a bot
// account backed by one of the plugin's LLM services.
type Agent struct{}

type AgentArgs struct {
	Username                string            `pulumi:"username" provider:"replaceOnChanges"`
	DisplayName             string            `pulumi:"displayName"`
	ServiceID               string            `pulumi:"serviceId"`
	Model                   string            `pulumi:"model,optional"`
	CustomInstructions      string            `pulumi:"customInstructions,optional"`
	ChannelAccessLevel      *AgentAccessLevel `pulumi:"channelAccessLevel,optional"`
	ChannelIDs              []string          `pulumi:"channelIds,optional"`
	UserAccessLevel         *AgentAccessLevel `pulumi:"userAccessLevel,optional"`
	UserIDs                 []string          `pulumi:"userIds,optional"`
	TeamIDs                 []string          `pulumi:"teamIds,optional"`
	AdminUserIDs            []string          `pulumi:"adminUserIds,optional"`
	EnableVision            bool              `pulumi:"enableVision,optional"`
	DisableTools            bool              `pulumi:"disableTools,optional"`
	EnabledNativeTools      []string          `pulumi:"enabledNativeTools,optional"`
	AutoEnableNewMCPTools   bool              `pulumi:"autoEnableNewMcpTools,optional"`
	MCPDynamicToolLoading   bool              `pulumi:"mcpDynamicToolLoading,optional"`
	ReasoningEnabled        bool              `pulumi:"reasoningEnabled,optional"`
	ReasoningEffort         string            `pulumi:"reasoningEffort,optional"`
	ThinkingBudget          int               `pulumi:"thinkingBudget,optional"`
	StructuredOutputEnabled bool              `pulumi:"structuredOutputEnabled,optional"`
	MaxToolTurns            int               `pulumi:"maxToolTurns,optional"`
}

type AgentState struct {
	AgentArgs
	BotUserID string `pulumi:"botUserId"`
}

func (r *Agent) Annotate(a infer.Annotator) {
	a.SetToken("index", "Agent")
	a.Describe(&r, "A self-service agent of the Agents plugin (mattermost-ai): a bot account backed by one of the plugin's LLM services, created through the plugin's agents endpoint. The plugin must be installed and enabled and the provider's account must be a system admin. Without an E20 or Enterprise license the plugin allows one agent per server. The resource ID is the agent ID; deleting it removes the agent and its bot account.")
}

func (args *AgentArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Username, "Username of the agent's bot account: a lowercase letter followed by lowercase letters, digits, dots, hyphens or underscores. Changing it replaces the agent.")
	a.Describe(&args.DisplayName, "Display name of the agent.")
	a.Describe(&args.ServiceID, "ID of the LLM service the agent uses, see AgentsConfig.")
	a.Describe(&args.Model, "Model to use; the service's default model when unset.")
	a.Describe(&args.CustomInstructions, "System prompt added to every conversation.")
	a.Describe(&args.ChannelAccessLevel, "Which channels may use the agent. Defaults to all.")
	a.Describe(&args.ChannelIDs, "Channel IDs the channel access level allows or blocks.")
	a.Describe(&args.UserAccessLevel, "Which users may talk to the agent. Defaults to all.")
	a.Describe(&args.UserIDs, "User IDs the user access level allows or blocks.")
	a.Describe(&args.TeamIDs, "Team IDs whose members may talk to the agent.")
	a.Describe(&args.AdminUserIDs, "User IDs that may edit the agent besides its creator and system admins.")
	a.Describe(&args.EnableVision, "Let the agent read images.")
	a.Describe(&args.DisableTools, "Disable tool calling.")
	a.Describe(&args.EnabledNativeTools, "Names of the plugin's built-in tools the agent may call; none when unset.")
	a.Describe(&args.AutoEnableNewMCPTools, "Give the agent every configured MCP tool, including ones added later.")
	a.Describe(&args.MCPDynamicToolLoading, "Let the agent discover and load MCP tools on demand instead of receiving every tool definition up front.")
	a.Describe(&args.ReasoningEnabled, "Enable extended reasoning where the model supports it.")
	a.Describe(&args.ReasoningEffort, "Reasoning effort for models that support it, for example `low`, `medium` or `high`.")
	a.Describe(&args.ThinkingBudget, "Thinking token budget for models that support it; 0 lets the plugin choose.")
	a.Describe(&args.StructuredOutputEnabled, "Ask the model for structured output.")
	a.Describe(&args.MaxToolTurns, "Maximum tool-calling turns per response; 0 lets the plugin choose.")
}

func (state *AgentState) Annotate(a infer.Annotator) {
	a.Describe(&state.BotUserID, "User ID of the agent's bot account.")
}

// WireDependencies keeps botUserId known while an agent is updated in a
// preview: the bot account only changes with the username, which replaces
// the agent. Without it every update would show dependents such as a
// TeamMember with an unknown user ID, which they would have to replace.
func (Agent) WireDependencies(f infer.FieldSelector, args *AgentArgs, state *AgentState) {
	f.OutputField(&state.BotUserID).DependsOn(f.InputField(&args.Username))
}

func (Agent) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[AgentArgs], error) {
	args, failures, err := infer.DefaultCheck[AgentArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[AgentArgs]{}, err
	}
	normalizeAgentLists(&args)
	return infer.CheckResponse[AgentArgs]{Inputs: args, Failures: failures}, nil
}

func (Agent) Create(ctx context.Context, req infer.CreateRequest[AgentArgs]) (infer.CreateResponse[AgentState], error) {
	state := AgentState{AgentArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[AgentState]{Output: state}, nil
	}
	body := agentRequestJSON(req.Inputs, agentJSON{})
	body["username"] = req.Inputs.Username
	var created agentJSON
	if err := client(ctx).PluginRequest(ctx, http.MethodPost, agentsPluginID, agentsPath, body, &created); err != nil {
		return infer.CreateResponse[AgentState]{}, fmt.Errorf("mattermost: creating agent %q: %w", req.Inputs.Username, err)
	}
	if created.ID == "" {
		return infer.CreateResponse[AgentState]{}, fmt.Errorf("mattermost: creating agent %q: the plugin returned no agent ID", req.Inputs.Username)
	}
	state.BotUserID = created.BotUserID
	return infer.CreateResponse[AgentState]{ID: created.ID, Output: state}, nil
}

func (Agent) Read(ctx context.Context, req infer.ReadRequest[AgentArgs, AgentState]) (infer.ReadResponse[AgentArgs, AgentState], error) {
	var agent agentJSON
	err := client(ctx).PluginRequest(ctx, http.MethodGet, agentsPluginID, agentsPath+"/"+req.ID, nil, &agent)
	if mm.IsPluginNotFound(err) {
		return infer.ReadResponse[AgentArgs, AgentState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[AgentArgs, AgentState]{}, fmt.Errorf("mattermost: reading agent %s: %w", req.ID, err)
	}
	inputs := agent.args()
	normalizeAgentLists(&inputs)
	return infer.ReadResponse[AgentArgs, AgentState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  AgentState{AgentArgs: inputs, BotUserID: firstNonEmpty(agent.BotUserID, req.State.BotUserID)},
	}, nil
}

// Update replaces the whole agent, as the plugin's endpoint does. The
// username stays out of the body because it cannot change, and the MCP tool
// selection is carried over from the server because it is not managed here.
func (Agent) Update(ctx context.Context, req infer.UpdateRequest[AgentArgs, AgentState]) (infer.UpdateResponse[AgentState], error) {
	state := AgentState{AgentArgs: req.Inputs, BotUserID: req.State.BotUserID}
	if req.DryRun {
		return infer.UpdateResponse[AgentState]{Output: state}, nil
	}
	var current agentJSON
	if err := client(ctx).PluginRequest(ctx, http.MethodGet, agentsPluginID, agentsPath+"/"+req.ID, nil, &current); err != nil {
		return infer.UpdateResponse[AgentState]{}, fmt.Errorf("mattermost: reading agent %s before updating it: %w", req.ID, err)
	}
	var updated agentJSON
	if err := client(ctx).PluginRequest(ctx, http.MethodPut, agentsPluginID, agentsPath+"/"+req.ID, agentRequestJSON(req.Inputs, current), &updated); err != nil {
		return infer.UpdateResponse[AgentState]{}, fmt.Errorf("mattermost: updating agent %s: %w", req.ID, err)
	}
	state.BotUserID = firstNonEmpty(updated.BotUserID, state.BotUserID)
	return infer.UpdateResponse[AgentState]{Output: state}, nil
}

func (Agent) Delete(ctx context.Context, req infer.DeleteRequest[AgentState]) (infer.DeleteResponse, error) {
	err := client(ctx).PluginRequest(ctx, http.MethodDelete, agentsPluginID, agentsPath+"/"+req.ID, nil, nil)
	if err != nil && !mm.IsPluginNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("mattermost: deleting agent %s: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// agentJSON is the plugin's BotConfig as its endpoints return it.
type agentJSON struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name"`
	DisplayName             string   `json:"displayName"`
	ServiceID               string   `json:"serviceID"`
	Model                   string   `json:"model"`
	CustomInstructions      string   `json:"customInstructions"`
	ChannelAccessLevel      int      `json:"channelAccessLevel"`
	ChannelIDs              []string `json:"channelIDs"`
	UserAccessLevel         int      `json:"userAccessLevel"`
	UserIDs                 []string `json:"userIDs"`
	TeamIDs                 []string `json:"teamIDs"`
	AdminUserIDs            []string `json:"adminUserIDs"`
	EnableVision            bool     `json:"enableVision"`
	DisableTools            bool     `json:"disableTools"`
	EnabledNativeTools      []string `json:"enabledNativeTools"`
	AutoEnableNewMCPTools   bool     `json:"autoEnableNewMCPTools"`
	ReasoningEnabled        bool     `json:"reasoningEnabled"`
	ReasoningEffort         string   `json:"reasoningEffort"`
	ThinkingBudget          int      `json:"thinkingBudget"`
	StructuredOutputEnabled bool     `json:"structuredOutputEnabled"`
	MaxToolTurns            int      `json:"maxToolTurns"`
	MCPDynamicToolLoading   bool     `json:"mcpDynamicToolLoading"`
	BotUserID               string   `json:"botUserID"`
	// Not managed; carried over on update.
	EnabledMCPTools []any `json:"enabledMCPTools"`
}

func (agent agentJSON) args() AgentArgs {
	return AgentArgs{
		Username:                agent.Name,
		DisplayName:             agent.DisplayName,
		ServiceID:               agent.ServiceID,
		Model:                   agent.Model,
		CustomInstructions:      agent.CustomInstructions,
		ChannelAccessLevel:      agentAccessLevelFromIndex(agent.ChannelAccessLevel),
		ChannelIDs:              agent.ChannelIDs,
		UserAccessLevel:         agentAccessLevelFromIndex(agent.UserAccessLevel),
		UserIDs:                 agent.UserIDs,
		TeamIDs:                 agent.TeamIDs,
		AdminUserIDs:            agent.AdminUserIDs,
		EnableVision:            agent.EnableVision,
		DisableTools:            agent.DisableTools,
		EnabledNativeTools:      agent.EnabledNativeTools,
		AutoEnableNewMCPTools:   agent.AutoEnableNewMCPTools,
		MCPDynamicToolLoading:   agent.MCPDynamicToolLoading,
		ReasoningEnabled:        agent.ReasoningEnabled,
		ReasoningEffort:         agent.ReasoningEffort,
		ThinkingBudget:          agent.ThinkingBudget,
		StructuredOutputEnabled: agent.StructuredOutputEnabled,
		MaxToolTurns:            agent.MaxToolTurns,
	}
}

// agentRequestJSON renders the create and update body of the agents
// endpoint. Lists are sent as empty arrays: the plugin stores the body as
// given, and a null list would read back differently from an empty one. The
// MCP tool selection comes from the agent as the server holds it.
func agentRequestJSON(args AgentArgs, current agentJSON) map[string]any {
	enabledMCPTools := current.EnabledMCPTools
	if enabledMCPTools == nil {
		enabledMCPTools = []any{}
	}
	return map[string]any{
		"displayName":             args.DisplayName,
		"serviceID":               args.ServiceID,
		"model":                   args.Model,
		"customInstructions":      args.CustomInstructions,
		"channelAccessLevel":      agentAccessLevelIndex(args.ChannelAccessLevel),
		"channelIDs":              nonNilList(args.ChannelIDs),
		"userAccessLevel":         agentAccessLevelIndex(args.UserAccessLevel),
		"userIDs":                 nonNilList(args.UserIDs),
		"teamIDs":                 nonNilList(args.TeamIDs),
		"adminUserIDs":            nonNilList(args.AdminUserIDs),
		"enableVision":            args.EnableVision,
		"disableTools":            args.DisableTools,
		"enabledNativeTools":      nonNilList(args.EnabledNativeTools),
		"autoEnableNewMCPTools":   args.AutoEnableNewMCPTools,
		"enabledMCPTools":         enabledMCPTools,
		"mcpDynamicToolLoading":   args.MCPDynamicToolLoading,
		"reasoningEnabled":        args.ReasoningEnabled,
		"reasoningEffort":         args.ReasoningEffort,
		"thinkingBudget":          args.ThinkingBudget,
		"structuredOutputEnabled": args.StructuredOutputEnabled,
		"maxToolTurns":            args.MaxToolTurns,
	}
}

// normalizeAgentLists turns empty lists into unset ones and unset access
// levels into `all`, so inputs that omit them keep matching what the plugin
// returns.
func normalizeAgentLists(args *AgentArgs) {
	for _, list := range []*[]string{&args.ChannelIDs, &args.UserIDs, &args.TeamIDs, &args.AdminUserIDs, &args.EnabledNativeTools} {
		if len(*list) == 0 {
			*list = nil
		}
	}
	for _, level := range []**AgentAccessLevel{&args.ChannelAccessLevel, &args.UserAccessLevel} {
		if *level == nil {
			all := AgentAccessAll
			*level = &all
		}
	}
}

func nonNilList(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
