package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

type OutgoingWebhook struct{}

type OutgoingWebhookArgs struct {
	TeamID       string   `pulumi:"teamId"`
	ChannelID    string   `pulumi:"channelId,optional"`
	DisplayName  string   `pulumi:"displayName"`
	Description  string   `pulumi:"description,optional"`
	TriggerWords []string `pulumi:"triggerWords,optional"`
	TriggerWhen  int      `pulumi:"triggerWhen,optional"`
	CallbackURLs []string `pulumi:"callbackUrls"`
	ContentType  string   `pulumi:"contentType,optional"`
	Username     string   `pulumi:"username,optional"`
	IconURL      string   `pulumi:"iconUrl,optional"`
}

type OutgoingWebhookState struct {
	OutgoingWebhookArgs
	CreatorID string `pulumi:"creatorId"`
	Token     string `pulumi:"token" provider:"secret"`
}

func (r *OutgoingWebhook) Annotate(a infer.Annotator) {
	a.SetToken("index", "OutgoingWebhook")
	a.Describe(&r, "A Mattermost outgoing webhook.")
}

func (OutgoingWebhook) Create(ctx context.Context, req infer.CreateRequest[OutgoingWebhookArgs]) (infer.CreateResponse[OutgoingWebhookState], error) {
	state := OutgoingWebhookState{OutgoingWebhookArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[OutgoingWebhookState]{Output: state}, nil
	}
	hook, _, err := client(ctx).API.CreateOutgoingWebhook(ctx, outgoingWebhookModel(req.Inputs, ""))
	if err != nil {
		return infer.CreateResponse[OutgoingWebhookState]{}, err
	}
	state.CreatorID = hook.CreatorId
	state.Token = hook.Token
	return infer.CreateResponse[OutgoingWebhookState]{ID: hook.Id, Output: state}, nil
}

func (OutgoingWebhook) Update(ctx context.Context, req infer.UpdateRequest[OutgoingWebhookArgs, OutgoingWebhookState]) (infer.UpdateResponse[OutgoingWebhookState], error) {
	state := OutgoingWebhookState{OutgoingWebhookArgs: req.Inputs, CreatorID: req.State.CreatorID, Token: req.State.Token}
	if req.DryRun {
		return infer.UpdateResponse[OutgoingWebhookState]{Output: state}, nil
	}
	hook, _, err := client(ctx).API.UpdateOutgoingWebhook(ctx, outgoingWebhookModel(req.Inputs, req.ID))
	if err != nil {
		return infer.UpdateResponse[OutgoingWebhookState]{}, err
	}
	state.CreatorID = hook.CreatorId
	state.Token = hook.Token
	return infer.UpdateResponse[OutgoingWebhookState]{Output: state}, nil
}

func (OutgoingWebhook) Read(ctx context.Context, req infer.ReadRequest[OutgoingWebhookArgs, OutgoingWebhookState]) (infer.ReadResponse[OutgoingWebhookArgs, OutgoingWebhookState], error) {
	hook, response, err := client(ctx).API.GetOutgoingWebhook(ctx, req.ID)
	if isNotFound(response) {
		return infer.ReadResponse[OutgoingWebhookArgs, OutgoingWebhookState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[OutgoingWebhookArgs, OutgoingWebhookState]{}, err
	}
	inputs := OutgoingWebhookArgs{
		TeamID: hook.TeamId, ChannelID: hook.ChannelId, DisplayName: hook.DisplayName,
		Description: hook.Description, TriggerWords: []string(hook.TriggerWords), TriggerWhen: hook.TriggerWhen,
		CallbackURLs: []string(hook.CallbackURLs), ContentType: hook.ContentType, Username: hook.Username, IconURL: hook.IconURL,
	}
	state := OutgoingWebhookState{OutgoingWebhookArgs: inputs, CreatorID: hook.CreatorId, Token: hook.Token}
	return infer.ReadResponse[OutgoingWebhookArgs, OutgoingWebhookState]{ID: hook.Id, Inputs: inputs, State: state}, nil
}

func (OutgoingWebhook) Delete(ctx context.Context, req infer.DeleteRequest[OutgoingWebhookState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.DeleteOutgoingWebhook(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func outgoingWebhookModel(args OutgoingWebhookArgs, id string) *model.OutgoingWebhook {
	return &model.OutgoingWebhook{
		Id: id, TeamId: args.TeamID, ChannelId: args.ChannelID, DisplayName: args.DisplayName,
		Description: args.Description, TriggerWords: model.StringArray(args.TriggerWords), TriggerWhen: args.TriggerWhen,
		CallbackURLs: model.StringArray(args.CallbackURLs), ContentType: args.ContentType, Username: args.Username, IconURL: args.IconURL,
	}
}
