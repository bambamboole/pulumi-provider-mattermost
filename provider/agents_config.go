package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

const (
	agentsPluginID   = "mattermost-ai"
	agentsConfigPath = "/admin/config"
)

// AgentsConfig manages the configuration of the Agents plugin
// (mattermost-ai): the LLM services and the plugin-wide switches. Since
// version 2.5 the plugin keeps this configuration in its own database tables
// behind /plugins/mattermost-ai/admin/config; values under
// PluginSettings.Plugins are only read by a one-time legacy migration.
type AgentsConfig struct{}

// AgentsService is an LLM service the agents talk to.
type AgentsService struct {
	ID                      string `pulumi:"id"`
	Name                    string `pulumi:"name"`
	Type                    string `pulumi:"type"`
	APIKey                  string `pulumi:"apiKey,optional" provider:"secret"`
	APIURL                  string `pulumi:"apiUrl,optional"`
	OrgID                   string `pulumi:"orgId,optional"`
	DefaultModel            string `pulumi:"defaultModel,optional"`
	InputTokenLimit         *int   `pulumi:"inputTokenLimit,optional"`
	OutputTokenLimit        *int   `pulumi:"outputTokenLimit,optional"`
	StreamingTimeoutSeconds *int   `pulumi:"streamingTimeoutSeconds,optional"`
	UseResponsesAPI         *bool  `pulumi:"useResponsesApi,optional"`
}

type AgentsConfigArgs struct {
	Services                        []AgentsService `pulumi:"services"`
	EnableTokenUsageLogging         *bool           `pulumi:"enableTokenUsageLogging,optional"`
	EnableCallSummary               *bool           `pulumi:"enableCallSummary,optional"`
	AllowedUpstreamHostnames        *string         `pulumi:"allowedUpstreamHostnames,optional"`
	AllowUnsafeLinks                *bool           `pulumi:"allowUnsafeLinks,optional"`
	EnableChannelMentionToolCalling *bool           `pulumi:"enableChannelMentionToolCalling,optional"`
	AllowNativeWebSearchInChannels  *bool           `pulumi:"allowNativeWebSearchInChannels,optional"`
	TranscriptBackend               *string         `pulumi:"transcriptBackend,optional"`
}

type AgentsConfigState struct {
	AgentsConfigArgs
}

func (r *AgentsConfig) Annotate(a infer.Annotator) {
	a.SetToken("index", "AgentsConfig")
	a.Describe(&r, "The configuration of the Agents plugin (mattermost-ai): its LLM services and plugin-wide switches, written to the plugin's admin configuration endpoint. The plugin must be installed and enabled, for example through a Plugin resource, and the provider's account must be a system admin. `services` replaces the plugin's service list; unset switches and everything else the endpoint holds (MCP servers, web search, embedding search, legacy bots) are left as they are. This is a singleton resource with ID `mattermost-ai`. Deleting the resource stops managing the configuration and keeps it on the server, because agents keep referring to the services.")
}

func (args *AgentsConfigArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Services, "The LLM services, replacing the plugin's list. Agents refer to a service by its `id`.")
	a.Describe(&args.EnableTokenUsageLogging, "Log token usage.")
	a.Describe(&args.EnableCallSummary, "Offer summaries of Calls recordings.")
	a.Describe(&args.AllowedUpstreamHostnames, "Comma-separated hostnames the agents may fetch, for example for link previews and tools.")
	a.Describe(&args.AllowUnsafeLinks, "Allow links to non-HTTPS or private hosts in agent output.")
	a.Describe(&args.EnableChannelMentionToolCalling, "Let agents call tools when mentioned in a channel.")
	a.Describe(&args.AllowNativeWebSearchInChannels, "Allow the LLM's native web search in channels.")
	a.Describe(&args.TranscriptBackend, "ID of the service that transcribes Calls recordings.")
}

func (service *AgentsService) Annotate(a infer.Annotator) {
	a.Describe(&service.ID, "ID of the service; agents refer to it.")
	a.Describe(&service.Name, "Display name of the service.")
	a.Describe(&service.Type, "Service type as the plugin names it, for example `openai`, `openaicompatible`, `anthropic`, `azure`, `bedrock` or `vertex`.")
	a.Describe(&service.APIKey, "API key of the service.")
	a.Describe(&service.APIURL, "Base URL of an OpenAI-compatible or self-hosted API, for example `https://openrouter.ai/api/v1`.")
	a.Describe(&service.OrgID, "Organization ID for OpenAI.")
	a.Describe(&service.DefaultModel, "Model used when an agent does not name one.")
	a.Describe(&service.InputTokenLimit, "Input token limit; 0 lets the plugin choose.")
	a.Describe(&service.OutputTokenLimit, "Output token limit; 0 lets the plugin choose.")
	a.Describe(&service.StreamingTimeoutSeconds, "Timeout of a streaming response in seconds; 0 lets the plugin choose.")
	a.Describe(&service.UseResponsesAPI, "Use OpenAI's Responses API instead of Chat Completions. The plugin forces it on for the `openai` type.")
}

func (AgentsConfig) Create(ctx context.Context, req infer.CreateRequest[AgentsConfigArgs]) (infer.CreateResponse[AgentsConfigState], error) {
	state := AgentsConfigState{AgentsConfigArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[AgentsConfigState]{ID: agentsPluginID, Output: state}, nil
	}
	if err := applyAgentsConfig(ctx, req.Inputs); err != nil {
		return infer.CreateResponse[AgentsConfigState]{}, err
	}
	return infer.CreateResponse[AgentsConfigState]{ID: agentsPluginID, Output: state}, nil
}

func (AgentsConfig) Update(ctx context.Context, req infer.UpdateRequest[AgentsConfigArgs, AgentsConfigState]) (infer.UpdateResponse[AgentsConfigState], error) {
	state := AgentsConfigState{AgentsConfigArgs: req.Inputs}
	if req.DryRun {
		return infer.UpdateResponse[AgentsConfigState]{Output: state}, nil
	}
	if err := applyAgentsConfig(ctx, req.Inputs); err != nil {
		return infer.UpdateResponse[AgentsConfigState]{}, err
	}
	return infer.UpdateResponse[AgentsConfigState]{Output: state}, nil
}

// Read supports import: with empty inputs every switch is imported; otherwise
// only the declared switches are refreshed. Services are always read back;
// an API key the endpoint does not return is kept from the inputs.
func (AgentsConfig) Read(ctx context.Context, req infer.ReadRequest[AgentsConfigArgs, AgentsConfigState]) (infer.ReadResponse[AgentsConfigArgs, AgentsConfigState], error) {
	current, err := readAgentsConfig(ctx)
	if mm.IsPluginNotFound(err) {
		return infer.ReadResponse[AgentsConfigArgs, AgentsConfigState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[AgentsConfigArgs, AgentsConfigState]{}, err
	}
	importAll := len(req.Inputs.Services) == 0 && req.Inputs.EnableTokenUsageLogging == nil && req.Inputs.EnableCallSummary == nil &&
		req.Inputs.AllowedUpstreamHostnames == nil && req.Inputs.AllowUnsafeLinks == nil && req.Inputs.EnableChannelMentionToolCalling == nil &&
		req.Inputs.AllowNativeWebSearchInChannels == nil && req.Inputs.TranscriptBackend == nil
	inputs := AgentsConfigArgs{Services: readAgentsServices(current, req.Inputs.Services)}
	readBool := func(target **bool, declared *bool, key string) {
		if declared == nil && !importAll {
			return
		}
		value, _ := current[key].(bool)
		*target = &value
	}
	readString := func(target **string, declared *string, key string) {
		if declared == nil && !importAll {
			return
		}
		value, _ := current[key].(string)
		*target = &value
	}
	readBool(&inputs.EnableTokenUsageLogging, req.Inputs.EnableTokenUsageLogging, "enableTokenUsageLogging")
	readBool(&inputs.EnableCallSummary, req.Inputs.EnableCallSummary, "enableCallSummary")
	readString(&inputs.AllowedUpstreamHostnames, req.Inputs.AllowedUpstreamHostnames, "allowedUpstreamHostnames")
	readBool(&inputs.AllowUnsafeLinks, req.Inputs.AllowUnsafeLinks, "allowUnsafeLinks")
	readBool(&inputs.EnableChannelMentionToolCalling, req.Inputs.EnableChannelMentionToolCalling, "enableChannelMentionToolCalling")
	readBool(&inputs.AllowNativeWebSearchInChannels, req.Inputs.AllowNativeWebSearchInChannels, "allowNativeWebSearchInChannels")
	readString(&inputs.TranscriptBackend, req.Inputs.TranscriptBackend, "transcriptBackend")
	return infer.ReadResponse[AgentsConfigArgs, AgentsConfigState]{
		ID:     agentsPluginID,
		Inputs: inputs,
		State:  AgentsConfigState{AgentsConfigArgs: inputs},
	}, nil
}

// Delete keeps the configuration: the agents on the server keep referring to
// the services, and removing the resource means "stop managing", as with
// SystemConfig.
func (AgentsConfig) Delete(context.Context, infer.DeleteRequest[AgentsConfigState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

func readAgentsConfig(ctx context.Context) (map[string]any, error) {
	current := map[string]any{}
	if err := client(ctx).PluginRequest(ctx, http.MethodGet, agentsPluginID, agentsConfigPath, nil, &current); err != nil {
		if mm.IsPluginNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("mattermost: reading the Agents configuration: %w", err)
	}
	return current, nil
}

// applyAgentsConfig overlays the declared values on the configuration the
// plugin holds and writes it back, so unmanaged parts survive.
func applyAgentsConfig(ctx context.Context, args AgentsConfigArgs) error {
	current, err := readAgentsConfig(ctx)
	if err != nil {
		if mm.IsPluginNotFound(err) {
			return fmt.Errorf("mattermost: the Agents plugin (%s) is not installed or not enabled: %w", agentsPluginID, err)
		}
		return err
	}
	current["services"] = agentsServicesJSON(args.Services)
	setBool := func(key string, value *bool) {
		if value != nil {
			current[key] = *value
		}
	}
	setString := func(key string, value *string) {
		if value != nil {
			current[key] = *value
		}
	}
	setBool("enableTokenUsageLogging", args.EnableTokenUsageLogging)
	setBool("enableCallSummary", args.EnableCallSummary)
	setString("allowedUpstreamHostnames", args.AllowedUpstreamHostnames)
	setBool("allowUnsafeLinks", args.AllowUnsafeLinks)
	setBool("enableChannelMentionToolCalling", args.EnableChannelMentionToolCalling)
	setBool("allowNativeWebSearchInChannels", args.AllowNativeWebSearchInChannels)
	setString("transcriptBackend", args.TranscriptBackend)
	if err := client(ctx).PluginRequest(ctx, http.MethodPut, agentsPluginID, agentsConfigPath, current, nil); err != nil {
		return fmt.Errorf("mattermost: writing the Agents configuration: %w", err)
	}
	return nil
}

// agentsServicesJSON renders the services as the plugin's ServiceConfig
// objects. Every service is sent completely, because the list replaces the
// plugin's list; unset limits are the plugin's zero values.
func agentsServicesJSON(services []AgentsService) []map[string]any {
	out := make([]map[string]any, 0, len(services))
	for _, service := range services {
		entry := map[string]any{
			"id":                      service.ID,
			"name":                    service.Name,
			"type":                    service.Type,
			"apiKey":                  service.APIKey,
			"apiURL":                  service.APIURL,
			"orgId":                   service.OrgID,
			"defaultModel":            service.DefaultModel,
			"tokenLimit":              intValue(service.InputTokenLimit),
			"outputTokenLimit":        intValue(service.OutputTokenLimit),
			"streamingTimeoutSeconds": intValue(service.StreamingTimeoutSeconds),
			"useResponsesAPI":         boolValue(service.UseResponsesAPI),
		}
		out = append(out, entry)
	}
	return out
}

// readAgentsServices maps the plugin's service list back to inputs. Limits
// and the Responses API switch are only read when declared, so that unset
// inputs keep matching the plugin's defaults; a missing API key is kept from
// the declared service of the same ID.
func readAgentsServices(current map[string]any, declared []AgentsService) []AgentsService {
	declaredByID := map[string]AgentsService{}
	for _, service := range declared {
		declaredByID[service.ID] = service
	}
	raw, _ := current["services"].([]any)
	if len(raw) == 0 {
		return nil
	}
	services := make([]AgentsService, 0, len(raw))
	for _, item := range raw {
		entry, _ := item.(map[string]any)
		id, _ := entry["id"].(string)
		known := declaredByID[id]
		service := AgentsService{
			ID:           id,
			Name:         stringField(entry, "name"),
			Type:         stringField(entry, "type"),
			APIKey:       stringField(entry, "apiKey"),
			APIURL:       stringField(entry, "apiURL"),
			OrgID:        stringField(entry, "orgId"),
			DefaultModel: stringField(entry, "defaultModel"),
		}
		if service.APIKey == "" {
			service.APIKey = known.APIKey
		}
		if known.InputTokenLimit != nil {
			service.InputTokenLimit = intField(entry, "tokenLimit")
		}
		if known.OutputTokenLimit != nil {
			service.OutputTokenLimit = intField(entry, "outputTokenLimit")
		}
		if known.StreamingTimeoutSeconds != nil {
			service.StreamingTimeoutSeconds = intField(entry, "streamingTimeoutSeconds")
		}
		if known.UseResponsesAPI != nil {
			value, _ := entry["useResponsesAPI"].(bool)
			service.UseResponsesAPI = &value
		}
		services = append(services, service)
	}
	return services
}

func stringField(entry map[string]any, key string) string {
	value, _ := entry[key].(string)
	return value
}

func intField(entry map[string]any, key string) *int {
	value := 0
	if number, ok := entry[key].(float64); ok {
		value = int(number)
	}
	return &value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func boolValue(value *bool) bool {
	return value != nil && *value
}
