package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

const (
	defaultBootstrapBotUsername      = "pulumi"
	defaultBootstrapTokenDescription = "pulumi"
)

// Bootstrap obtains a system-admin bot token for a Mattermost server without
// user interaction. On a fresh server it signs up the first account, which
// Mattermost promotes to system admin; on a running server it logs in with
// the admin credentials or uses an existing admin token. It then creates or
// adopts the bot, applies its system roles and issues an access token.
type Bootstrap struct{}

type BootstrapArgs struct {
	BaseURL          string       `pulumi:"baseUrl,optional"`
	AdminUsername    string       `pulumi:"adminUsername"`
	AdminEmail       string       `pulumi:"adminEmail"`
	AdminPassword    string       `pulumi:"adminPassword,optional" provider:"secret"`
	AdminToken       string       `pulumi:"adminToken,optional" provider:"secret"`
	BotUsername      string       `pulumi:"botUsername,optional" provider:"replaceOnChanges"`
	BotDisplayName   string       `pulumi:"botDisplayName,optional"`
	BotDescription   string       `pulumi:"botDescription,optional"`
	Roles            []SystemRole `pulumi:"roles,optional"`
	TokenDescription string       `pulumi:"tokenDescription,optional"`
}

type BootstrapState struct {
	BootstrapArgs
	AdminUserID string `pulumi:"adminUserId"`
	BotUserID   string `pulumi:"botUserId"`
	TokenID     string `pulumi:"tokenId"`
	Token       string `pulumi:"token" provider:"secret"`
}

func (r *Bootstrap) Annotate(a infer.Annotator) {
	a.SetToken("index", "Bootstrap")
	a.Describe(&r, "Obtains a system-admin bot token without user interaction. On a fresh server the admin account is signed up as the first user (which Mattermost promotes to system admin); on a running server the admin credentials or an existing admin token are used. The bot is created or adopted, its system roles applied and an access token issued. The resource authenticates on its own, so its provider does not need a token. The resource ID is the bot's user ID.")
}

func (args *BootstrapArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.BaseURL, "Base URL of the Mattermost instance. Defaults to the provider's base URL.")
	a.Describe(&args.AdminUsername, "Username of the bootstrap admin. Created on a fresh server, used for login otherwise.")
	a.Describe(&args.AdminEmail, "Email of the bootstrap admin. Only used when the account is created.")
	a.Describe(&args.AdminPassword, "Password of the bootstrap admin. Required unless adminToken is set.")
	a.Describe(&args.AdminToken, "Existing system-admin token (personal access or bot token) used instead of a password login.")
	a.Describe(&args.BotUsername, "Username of the bot to create or adopt. Defaults to \"pulumi\".")
	a.Describe(&args.BotDisplayName, "Display name of the bot.")
	a.Describe(&args.BotDescription, "Description of the bot.")
	a.Describe(&args.Roles, "System roles applied to the bot account. Defaults to [\"system_user\", \"system_admin\", \"system_post_all\"].")
	a.Describe(&args.TokenDescription, "Description of the bot access token. Changing it rotates the token. Defaults to \"pulumi\".")
	a.SetDefault(&args.BotUsername, defaultBootstrapBotUsername)
	a.SetDefault(&args.TokenDescription, defaultBootstrapTokenDescription)
}

func (state *BootstrapState) Annotate(a infer.Annotator) {
	a.Describe(&state.AdminUserID, "User ID of the bootstrap admin.")
	a.Describe(&state.BotUserID, "User ID of the bot account.")
	a.Describe(&state.TokenID, "ID of the issued access token.")
	a.Describe(&state.Token, "The issued access token. Use it as the token of a second provider instance.")
}

func defaultBootstrapRoles() []SystemRole {
	return normalizeRoles([]SystemRole{SystemRoleUser, SystemRoleAdmin, SystemRolePostAll})
}

func (Bootstrap) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[BootstrapArgs], error) {
	args, failures, err := infer.DefaultCheck[BootstrapArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[BootstrapArgs]{}, err
	}
	if args.BotUsername == "" {
		args.BotUsername = defaultBootstrapBotUsername
	}
	if args.TokenDescription == "" {
		args.TokenDescription = defaultBootstrapTokenDescription
	}
	if len(args.Roles) == 0 {
		args.Roles = defaultBootstrapRoles()
	} else {
		args.Roles = normalizeRoles(args.Roles)
	}
	if strings.TrimSpace(args.AdminPassword) == "" && strings.TrimSpace(args.AdminToken) == "" {
		failures = append(failures, p.CheckFailure{Property: "adminPassword", Reason: "either adminPassword or adminToken must be set"})
	}
	return infer.CheckResponse[BootstrapArgs]{Inputs: args, Failures: failures}, nil
}

func (Bootstrap) Create(ctx context.Context, req infer.CreateRequest[BootstrapArgs]) (infer.CreateResponse[BootstrapState], error) {
	state := BootstrapState{BootstrapArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[BootstrapState]{Output: state}, nil
	}
	baseURL := bootstrapBaseURL(ctx, req.Inputs)
	admin, adminID, err := bootstrapAdminSession(ctx, baseURL, req.Inputs)
	if err != nil {
		return infer.CreateResponse[BootstrapState]{}, err
	}
	state.AdminUserID = adminID

	bot, err := bootstrapEnsureBot(ctx, admin, req.Inputs)
	if err != nil {
		return infer.CreateResponse[BootstrapState]{}, err
	}
	state.BotUserID = bot.UserId
	state.BotDisplayName = bot.DisplayName
	state.BotDescription = bot.Description

	if err := bootstrapEnsureRoles(ctx, admin, bot.UserId, req.Inputs.Roles); err != nil {
		return infer.CreateResponse[BootstrapState]{}, err
	}

	token, _, err := admin.API.CreateUserAccessToken(ctx, bot.UserId, req.Inputs.TokenDescription, 0)
	if err != nil {
		return infer.CreateResponse[BootstrapState]{}, fmt.Errorf("mattermost: creating access token for bot %q: %w", req.Inputs.BotUsername, err)
	}
	state.TokenID = token.Id
	state.Token = token.Token
	return infer.CreateResponse[BootstrapState]{ID: bot.UserId, Output: state}, nil
}

func (Bootstrap) Read(ctx context.Context, req infer.ReadRequest[BootstrapArgs, BootstrapState]) (infer.ReadResponse[BootstrapArgs, BootstrapState], error) {
	state := req.State
	if state.Token == "" {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, nil
	}
	client, err := mm.New(bootstrapBaseURL(ctx, state.BootstrapArgs), state.Token)
	if err != nil {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, err
	}
	me, response, err := client.API.GetMe(ctx, "")
	if isUnauthorized(response) || isNotFound(response) {
		// The token was revoked or the bot deleted: recreate on the next update.
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, err
	}
	if me.Id != state.BotUserID || me.DeleteAt > 0 {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, nil
	}
	bot, response, err := client.API.GetBot(ctx, state.BotUserID, "")
	if isNotFound(response) {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, err
	}

	inputs := req.Inputs
	if inputs.AdminUsername == "" {
		inputs = state.BootstrapArgs
	}
	inputs.BotUsername = bot.Username
	inputs.BotDisplayName = bot.DisplayName
	inputs.BotDescription = bot.Description
	inputs.Roles = parseRoles(me.Roles)
	state.BootstrapArgs = inputs
	return infer.ReadResponse[BootstrapArgs, BootstrapState]{ID: req.ID, Inputs: inputs, State: state}, nil
}

func (Bootstrap) Update(ctx context.Context, req infer.UpdateRequest[BootstrapArgs, BootstrapState]) (infer.UpdateResponse[BootstrapState], error) {
	state := BootstrapState{
		BootstrapArgs: req.Inputs,
		AdminUserID:   req.State.AdminUserID,
		BotUserID:     req.State.BotUserID,
		TokenID:       req.State.TokenID,
		Token:         req.State.Token,
	}
	if req.DryRun {
		return infer.UpdateResponse[BootstrapState]{Output: state}, nil
	}
	client, err := bootstrapManagementSession(ctx, req.State, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[BootstrapState]{}, err
	}

	if req.Inputs.BotDisplayName != req.State.BotDisplayName || req.Inputs.BotDescription != req.State.BotDescription {
		displayName := req.Inputs.BotDisplayName
		description := req.Inputs.BotDescription
		if _, _, err := client.API.PatchBot(ctx, state.BotUserID, &model.BotPatch{DisplayName: &displayName, Description: &description}); err != nil {
			return infer.UpdateResponse[BootstrapState]{}, err
		}
	}
	if !rolesEqual(req.State.Roles, req.Inputs.Roles) {
		if _, err := client.API.UpdateUserRoles(ctx, state.BotUserID, joinRoles(req.Inputs.Roles)); err != nil {
			return infer.UpdateResponse[BootstrapState]{}, err
		}
	}
	if req.Inputs.TokenDescription != req.State.TokenDescription {
		token, _, err := client.API.CreateUserAccessToken(ctx, state.BotUserID, req.Inputs.TokenDescription, 0)
		if err != nil {
			return infer.UpdateResponse[BootstrapState]{}, fmt.Errorf("mattermost: rotating access token: %w", err)
		}
		state.TokenID = token.Id
		state.Token = token.Token
		if response, err := client.API.RevokeUserAccessToken(ctx, req.State.TokenID); err != nil && !isNotFound(response) {
			return infer.UpdateResponse[BootstrapState]{}, fmt.Errorf("mattermost: revoking previous access token: %w", err)
		}
	}
	return infer.UpdateResponse[BootstrapState]{Output: state}, nil
}

// Delete revokes the access token. The bot and the admin account are kept:
// they are cheap to reuse and deleting them would lock other integrations out.
func (Bootstrap) Delete(ctx context.Context, req infer.DeleteRequest[BootstrapState]) (infer.DeleteResponse, error) {
	if req.State.TokenID == "" {
		return infer.DeleteResponse{}, nil
	}
	client, err := bootstrapManagementSession(ctx, req.State, req.State.BootstrapArgs)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	response, err := client.API.RevokeUserAccessToken(ctx, req.State.TokenID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func bootstrapBaseURL(ctx context.Context, args BootstrapArgs) string {
	if strings.TrimSpace(args.BaseURL) != "" {
		return args.BaseURL
	}
	return client(ctx).BaseURL
}

// bootstrapAdminSession returns an authenticated client for the bootstrap
// admin and its user ID. An admin token wins over a password login; on a
// server without any accounts the admin is signed up first.
func bootstrapAdminSession(ctx context.Context, baseURL string, args BootstrapArgs) (*mm.Client, string, error) {
	if strings.TrimSpace(args.AdminToken) != "" {
		client, err := mm.New(baseURL, args.AdminToken)
		if err != nil {
			return nil, "", err
		}
		me, _, err := client.API.GetMe(ctx, "")
		if err != nil {
			return nil, "", fmt.Errorf("mattermost: adminToken was rejected: %w", err)
		}
		return client, me.Id, nil
	}

	anonymous, err := mm.NewAnonymous(baseURL)
	if err != nil {
		return nil, "", err
	}
	user, _, loginErr := anonymous.API.Login(ctx, args.AdminUsername, args.AdminPassword)
	if loginErr == nil {
		return anonymous, user.Id, nil
	}

	config, _, err := anonymous.API.GetClientConfig(ctx, "")
	if err != nil {
		return nil, "", fmt.Errorf("mattermost: login as %q failed (%w) and the client configuration could not be read: %w", args.AdminUsername, loginErr, err)
	}
	if config["NoAccounts"] != "true" {
		return nil, "", fmt.Errorf("mattermost: login as %q failed and the server already has accounts; check adminPassword or provide adminToken: %w", args.AdminUsername, loginErr)
	}
	if _, _, err := anonymous.API.CreateUser(ctx, &model.User{
		Username: args.AdminUsername,
		Email:    args.AdminEmail,
		Password: args.AdminPassword,
	}); err != nil {
		return nil, "", fmt.Errorf("mattermost: signing up the first admin %q: %w", args.AdminUsername, err)
	}
	user, _, err = anonymous.API.Login(ctx, args.AdminUsername, args.AdminPassword)
	if err != nil {
		return nil, "", fmt.Errorf("mattermost: login as the freshly created admin %q: %w", args.AdminUsername, err)
	}
	return anonymous, user.Id, nil
}

// bootstrapManagementSession prefers the issued bot token, which carries the
// declared roles, and falls back to the admin credentials when the token no
// longer works.
func bootstrapManagementSession(ctx context.Context, state BootstrapState, args BootstrapArgs) (*mm.Client, error) {
	baseURL := bootstrapBaseURL(ctx, args)
	if state.Token != "" {
		client, err := mm.New(baseURL, state.Token)
		if err == nil {
			if _, _, err := client.API.GetMe(ctx, ""); err == nil {
				return client, nil
			}
		}
	}
	client, _, err := bootstrapAdminSession(ctx, baseURL, args)
	return client, err
}

func bootstrapEnsureBot(ctx context.Context, admin *mm.Client, args BootstrapArgs) (*model.Bot, error) {
	user, response, err := admin.API.GetUserByUsername(ctx, args.BotUsername, "")
	switch {
	case err == nil:
		if !user.IsBot {
			return nil, fmt.Errorf("mattermost: user %q exists and is not a bot; choose another botUsername", args.BotUsername)
		}
		bot, _, err := admin.API.GetBot(ctx, user.Id, "")
		if err != nil {
			return nil, err
		}
		if (args.BotDisplayName != "" && args.BotDisplayName != bot.DisplayName) ||
			(args.BotDescription != "" && args.BotDescription != bot.Description) {
			displayName := firstNonEmpty(args.BotDisplayName, bot.DisplayName)
			description := firstNonEmpty(args.BotDescription, bot.Description)
			bot, _, err = admin.API.PatchBot(ctx, user.Id, &model.BotPatch{DisplayName: &displayName, Description: &description})
			if err != nil {
				return nil, err
			}
		}
		return bot, nil
	case isNotFound(response):
		bot, _, err := admin.API.CreateBot(ctx, &model.Bot{
			Username:    args.BotUsername,
			DisplayName: args.BotDisplayName,
			Description: args.BotDescription,
		})
		if err != nil {
			return nil, fmt.Errorf("mattermost: creating bot %q: %w", args.BotUsername, err)
		}
		return bot, nil
	default:
		return nil, err
	}
}

func bootstrapEnsureRoles(ctx context.Context, admin *mm.Client, userID string, roles []SystemRole) error {
	user, _, err := admin.API.GetUser(ctx, userID, "")
	if err != nil {
		return err
	}
	if rolesEqual(parseRoles(user.Roles), roles) {
		return nil
	}
	if _, err := admin.API.UpdateUserRoles(ctx, userID, joinRoles(roles)); err != nil {
		return fmt.Errorf("mattermost: applying roles to bot: %w", err)
	}
	return nil
}

func isUnauthorized(response *model.Response) bool {
	return response != nil && response.StatusCode == http.StatusUnauthorized
}
