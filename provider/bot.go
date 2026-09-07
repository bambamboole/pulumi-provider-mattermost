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
	// System roles of the bot's user account. Unset leaves the roles unmanaged.
	Roles []SystemRole `pulumi:"roles,optional"`
}

type BotState struct {
	BotArgs
	UserID  string `pulumi:"userId"`
	OwnerID string `pulumi:"ownerId"`
}

func (r *Bot) Annotate(a infer.Annotator) {
	a.SetToken("index", "Bot")
	a.Describe(&r, "A Mattermost bot account. A bot is a user account with a bot flag: the bot API owns username, display name and description, while system roles live on the user account and are applied through the roles endpoint when `roles` is set.")
}

func (args *BotArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Roles, "System roles applied to the bot's user account, e.g. [\"system_user\", \"system_admin\", \"system_post_all\"]. When unset, the roles are left as they are and never read back, so existing bots keep the roles granted outside Pulumi.")
}

// rolesManaged reports whether the declaration takes ownership of the roles.
func (args BotArgs) rolesManaged() bool {
	return len(args.Roles) > 0
}

func (Bot) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[BotArgs], error) {
	args, failures, err := infer.DefaultCheck[BotArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[BotArgs]{}, err
	}
	if args.rolesManaged() {
		args.Roles = normalizeRoles(args.Roles)
	}
	return infer.CheckResponse[BotArgs]{Inputs: args, Failures: failures}, nil
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
	// Mattermost creates bot accounts with system_user only.
	if req.Inputs.rolesManaged() && !rolesEqual([]SystemRole{SystemRoleUser}, req.Inputs.Roles) {
		if _, err := client(ctx).API.UpdateUserRoles(ctx, bot.UserId, joinRoles(normalizeRoles(req.Inputs.Roles))); err != nil {
			return infer.CreateResponse[BotState]{}, err
		}
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
	if req.Inputs.rolesManaged() && !rolesEqual(req.State.Roles, req.Inputs.Roles) {
		if _, err := client(ctx).API.UpdateUserRoles(ctx, req.ID, joinRoles(normalizeRoles(req.Inputs.Roles))); err != nil {
			return infer.UpdateResponse[BotState]{}, err
		}
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
	// The bot API does not return roles; only look at the user account when
	// the declaration manages them, so unmanaged roles never show up as drift.
	if req.Inputs.rolesManaged() {
		user, _, err := client(ctx).API.GetUser(ctx, bot.UserId, "")
		if err != nil {
			return infer.ReadResponse[BotArgs, BotState]{}, err
		}
		inputs.Roles = parseRoles(user.Roles)
	}
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
