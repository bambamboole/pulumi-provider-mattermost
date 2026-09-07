package provider

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// AccessToken issues a personal access token for a user or bot account. The
// token value is only returned once by Mattermost and is kept in state as a
// secret, so it can feed other resources such as a Worker secret.
type AccessToken struct{}

type AccessTokenArgs struct {
	UserID      string `pulumi:"userId" provider:"replaceOnChanges"`
	Description string `pulumi:"description" provider:"replaceOnChanges"`
}

type AccessTokenState struct {
	AccessTokenArgs
	Token string `pulumi:"token" provider:"secret"`
}

func (r *AccessToken) Annotate(a infer.Annotator) {
	a.SetToken("index", "AccessToken")
	a.Describe(&r, "A personal access token of a user or bot account. Bot tokens are exempt from EnableUserAccessTokens; user tokens need it. The provider's account needs edit_other_users (system admin) to issue tokens for other accounts. Changing the user or the description replaces the token; a revoked or disabled token is recreated on the next update. The resource ID is the token ID.")
}

func (args *AccessTokenArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.UserID, "ID of the user or bot account the token belongs to (for bots, the bot's userId).")
	a.Describe(&args.Description, "Description shown in Mattermost next to the token.")
}

func (state *AccessTokenState) Annotate(a infer.Annotator) {
	a.Describe(&state.Token, "The token value. Only available from the create response and kept in state.")
}

func (AccessToken) Create(ctx context.Context, req infer.CreateRequest[AccessTokenArgs]) (infer.CreateResponse[AccessTokenState], error) {
	state := AccessTokenState{AccessTokenArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[AccessTokenState]{Output: state}, nil
	}
	token, _, err := client(ctx).API.CreateUserAccessToken(ctx, req.Inputs.UserID, req.Inputs.Description, 0)
	if err != nil {
		return infer.CreateResponse[AccessTokenState]{}, fmt.Errorf("mattermost: creating access token for %s: %w", req.Inputs.UserID, err)
	}
	state.Token = token.Token
	return infer.CreateResponse[AccessTokenState]{ID: token.Id, Output: state}, nil
}

// Read checks that the token still exists and is active. Mattermost never
// returns the token value again, so it is carried over from the state.
func (AccessToken) Read(ctx context.Context, req infer.ReadRequest[AccessTokenArgs, AccessTokenState]) (infer.ReadResponse[AccessTokenArgs, AccessTokenState], error) {
	token, response, err := client(ctx).API.GetUserAccessToken(ctx, req.ID)
	if isNotFound(response) {
		return infer.ReadResponse[AccessTokenArgs, AccessTokenState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[AccessTokenArgs, AccessTokenState]{}, err
	}
	if !token.IsActive {
		return infer.ReadResponse[AccessTokenArgs, AccessTokenState]{}, nil
	}
	inputs := AccessTokenArgs{UserID: token.UserId, Description: token.Description}
	return infer.ReadResponse[AccessTokenArgs, AccessTokenState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  AccessTokenState{AccessTokenArgs: inputs, Token: req.State.Token},
	}, nil
}

func (AccessToken) Delete(ctx context.Context, req infer.DeleteRequest[AccessTokenState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.RevokeUserAccessToken(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
