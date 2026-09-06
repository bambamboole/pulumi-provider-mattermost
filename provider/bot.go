package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Bot manages a Mattermost bot account.
type Bot struct{}

type BotArgs struct {
	Username    string `pulumi:"username"`
	DisplayName string `pulumi:"displayName,optional"`
	Description string `pulumi:"description,optional"`
}

type BotState struct {
	BotArgs
	UserID  string `pulumi:"userId"`
	OwnerID string `pulumi:"ownerId"`
}

func (r *Bot) Annotate(a infer.Annotator) {
	a.SetToken("index", "Bot")
	a.Describe(&r, "A Mattermost bot account.")
}

func (Bot) Create(ctx context.Context, req infer.CreateRequest[BotArgs]) (infer.CreateResponse[BotState], error) {
	state := BotState{BotArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[BotState]{Output: state}, nil
	}
	bot, _, err := client(ctx).API.CreateBot(ctx, &model.Bot{
		Username:    req.Inputs.Username,
		DisplayName: req.Inputs.DisplayName,
		Description: req.Inputs.Description,
	})
	if err != nil {
		return infer.CreateResponse[BotState]{}, err
	}
	state.UserID = bot.UserId
	state.OwnerID = bot.OwnerId
	return infer.CreateResponse[BotState]{ID: bot.UserId, Output: state}, nil
}

func (Bot) Update(ctx context.Context, req infer.UpdateRequest[BotArgs, BotState]) (infer.UpdateResponse[BotState], error) {
	state := BotState{BotArgs: req.Inputs, UserID: req.ID, OwnerID: req.State.OwnerID}
	if req.DryRun {
		return infer.UpdateResponse[BotState]{Output: state}, nil
	}
	username := req.Inputs.Username
	displayName := req.Inputs.DisplayName
	description := req.Inputs.Description
	bot, _, err := client(ctx).API.PatchBot(ctx, req.ID, &model.BotPatch{
		Username:    &username,
		DisplayName: &displayName,
		Description: &description,
	})
	if err != nil {
		return infer.UpdateResponse[BotState]{}, err
	}
	state.OwnerID = bot.OwnerId
	return infer.UpdateResponse[BotState]{Output: state}, nil
}

func (Bot) Read(ctx context.Context, req infer.ReadRequest[BotArgs, BotState]) (infer.ReadResponse[BotArgs, BotState], error) {
	bot, response, err := client(ctx).API.GetBot(ctx, req.ID, "")
	if isNotFound(response) {
		return infer.ReadResponse[BotArgs, BotState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[BotArgs, BotState]{}, err
	}
	inputs := BotArgs{Username: bot.Username, DisplayName: bot.DisplayName, Description: bot.Description}
	return infer.ReadResponse[BotArgs, BotState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  BotState{BotArgs: inputs, UserID: bot.UserId, OwnerID: bot.OwnerId},
	}, nil
}

func (Bot) Delete(ctx context.Context, req infer.DeleteRequest[BotState]) (infer.DeleteResponse, error) {
	_, response, err := client(ctx).API.DisableBot(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
