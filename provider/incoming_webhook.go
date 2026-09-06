package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

type IncomingWebhook struct{}

type IncomingWebhookArgs struct {
	ChannelID     string `pulumi:"channelId"`
	DisplayName   string `pulumi:"displayName"`
	Description   string `pulumi:"description,optional"`
	Username      string `pulumi:"username,optional"`
	IconURL       string `pulumi:"iconUrl,optional"`
	ChannelLocked bool   `pulumi:"channelLocked,optional"`
}

type IncomingWebhookState struct {
	IncomingWebhookArgs
	ID     string `pulumi:"id"`
	TeamID string `pulumi:"teamId"`
	UserID string `pulumi:"userId"`
}

func (r *IncomingWebhook) Annotate(a infer.Annotator) {
	a.SetToken("index", "IncomingWebhook")
	a.Describe(&r, "A Mattermost incoming webhook bound to a channel.")
}

func (IncomingWebhook) Create(ctx context.Context, req infer.CreateRequest[IncomingWebhookArgs]) (infer.CreateResponse[IncomingWebhookState], error) {
	state := IncomingWebhookState{IncomingWebhookArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[IncomingWebhookState]{Output: state}, nil
	}
	hook, _, err := client(ctx).API.CreateIncomingWebhook(ctx, &model.IncomingWebhook{
		ChannelId:     req.Inputs.ChannelID,
		DisplayName:   req.Inputs.DisplayName,
		Description:   req.Inputs.Description,
		Username:      req.Inputs.Username,
		IconURL:       req.Inputs.IconURL,
		ChannelLocked: req.Inputs.ChannelLocked,
	})
	if err != nil {
		return infer.CreateResponse[IncomingWebhookState]{}, err
	}
	state.ID = hook.Id
	state.TeamID = hook.TeamId
	state.UserID = hook.UserId
	return infer.CreateResponse[IncomingWebhookState]{ID: hook.Id, Output: state}, nil
}

func (IncomingWebhook) Update(ctx context.Context, req infer.UpdateRequest[IncomingWebhookArgs, IncomingWebhookState]) (infer.UpdateResponse[IncomingWebhookState], error) {
	state := IncomingWebhookState{IncomingWebhookArgs: req.Inputs, ID: req.ID, TeamID: req.State.TeamID, UserID: req.State.UserID}
	if req.DryRun {
		return infer.UpdateResponse[IncomingWebhookState]{Output: state}, nil
	}
	hook, _, err := client(ctx).API.UpdateIncomingWebhook(ctx, &model.IncomingWebhook{
		Id:            req.ID,
		ChannelId:     req.Inputs.ChannelID,
		DisplayName:   req.Inputs.DisplayName,
		Description:   req.Inputs.Description,
		Username:      req.Inputs.Username,
		IconURL:       req.Inputs.IconURL,
		ChannelLocked: req.Inputs.ChannelLocked,
	})
	if err != nil {
		return infer.UpdateResponse[IncomingWebhookState]{}, err
	}
	state.TeamID = hook.TeamId
	state.UserID = hook.UserId
	return infer.UpdateResponse[IncomingWebhookState]{Output: state}, nil
}

func (IncomingWebhook) Read(ctx context.Context, req infer.ReadRequest[IncomingWebhookArgs, IncomingWebhookState]) (infer.ReadResponse[IncomingWebhookArgs, IncomingWebhookState], error) {
	hook, response, err := client(ctx).API.GetIncomingWebhook(ctx, req.ID, "")
	if isNotFound(response) {
		return infer.ReadResponse[IncomingWebhookArgs, IncomingWebhookState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[IncomingWebhookArgs, IncomingWebhookState]{}, err
	}
	inputs := IncomingWebhookArgs{ChannelID: hook.ChannelId, DisplayName: hook.DisplayName, Description: hook.Description, Username: hook.Username, IconURL: hook.IconURL, ChannelLocked: hook.ChannelLocked}
	state := IncomingWebhookState{IncomingWebhookArgs: inputs, ID: hook.Id, TeamID: hook.TeamId, UserID: hook.UserId}
	return infer.ReadResponse[IncomingWebhookArgs, IncomingWebhookState]{ID: hook.Id, Inputs: inputs, State: state}, nil
}

func (IncomingWebhook) Delete(ctx context.Context, req infer.DeleteRequest[IncomingWebhookState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.DeleteIncomingWebhook(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
