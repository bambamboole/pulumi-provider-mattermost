package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// OAuthApp manages a Mattermost OAuth 2.0 client application.
type OAuthApp struct{}

type OAuthAppArgs struct {
	Name         string   `pulumi:"name"`
	Description  string   `pulumi:"description,optional"`
	IconURL      string   `pulumi:"iconUrl,optional"`
	CallbackURLs []string `pulumi:"callbackUrls"`
	Homepage     string   `pulumi:"homepage,optional"`
	IsTrusted    bool     `pulumi:"isTrusted,optional"`
}

type OAuthAppState struct {
	OAuthAppArgs
	ClientSecret string `pulumi:"clientSecret" provider:"secret"`
	CreatorID    string `pulumi:"creatorId"`
}

func (r *OAuthApp) Annotate(a infer.Annotator) {
	a.SetToken("index", "OAuthApp")
	a.Describe(&r, "A Mattermost OAuth 2.0 client application.")
}

func (OAuthApp) Create(ctx context.Context, req infer.CreateRequest[OAuthAppArgs]) (infer.CreateResponse[OAuthAppState], error) {
	state := OAuthAppState{OAuthAppArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[OAuthAppState]{Output: state}, nil
	}
	app, _, err := client(ctx).API.CreateOAuthApp(ctx, &model.OAuthApp{
		Name:         req.Inputs.Name,
		Description:  req.Inputs.Description,
		IconURL:      req.Inputs.IconURL,
		CallbackUrls: model.StringArray(req.Inputs.CallbackURLs),
		Homepage:     req.Inputs.Homepage,
		IsTrusted:    req.Inputs.IsTrusted,
	})
	if err != nil {
		return infer.CreateResponse[OAuthAppState]{}, err
	}
	state.ClientSecret = app.ClientSecret
	state.CreatorID = app.CreatorId
	return infer.CreateResponse[OAuthAppState]{ID: app.Id, Output: state}, nil
}

func (OAuthApp) Update(ctx context.Context, req infer.UpdateRequest[OAuthAppArgs, OAuthAppState]) (infer.UpdateResponse[OAuthAppState], error) {
	state := OAuthAppState{OAuthAppArgs: req.Inputs, ClientSecret: req.State.ClientSecret, CreatorID: req.State.CreatorID}
	if req.DryRun {
		return infer.UpdateResponse[OAuthAppState]{Output: state}, nil
	}
	app, _, err := client(ctx).API.UpdateOAuthApp(ctx, &model.OAuthApp{
		Id:           req.ID,
		ClientSecret: req.State.ClientSecret,
		Name:         req.Inputs.Name,
		Description:  req.Inputs.Description,
		IconURL:      req.Inputs.IconURL,
		CallbackUrls: model.StringArray(req.Inputs.CallbackURLs),
		Homepage:     req.Inputs.Homepage,
		IsTrusted:    req.Inputs.IsTrusted,
	})
	if err != nil {
		return infer.UpdateResponse[OAuthAppState]{}, err
	}
	if app.ClientSecret != "" {
		state.ClientSecret = app.ClientSecret
	}
	state.CreatorID = app.CreatorId
	return infer.UpdateResponse[OAuthAppState]{Output: state}, nil
}

func (OAuthApp) Read(ctx context.Context, req infer.ReadRequest[OAuthAppArgs, OAuthAppState]) (infer.ReadResponse[OAuthAppArgs, OAuthAppState], error) {
	app, response, err := client(ctx).API.GetOAuthApp(ctx, req.ID)
	if isNotFound(response) {
		return infer.ReadResponse[OAuthAppArgs, OAuthAppState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[OAuthAppArgs, OAuthAppState]{}, err
	}
	inputs := OAuthAppArgs{
		Name:         app.Name,
		Description:  app.Description,
		IconURL:      app.IconURL,
		CallbackURLs: []string(app.CallbackUrls),
		Homepage:     app.Homepage,
		IsTrusted:    app.IsTrusted,
	}
	secret := app.ClientSecret
	if secret == "" {
		secret = req.State.ClientSecret
	}
	return infer.ReadResponse[OAuthAppArgs, OAuthAppState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  OAuthAppState{OAuthAppArgs: inputs, ClientSecret: secret, CreatorID: app.CreatorId},
	}, nil
}

func (OAuthApp) Delete(ctx context.Context, req infer.DeleteRequest[OAuthAppState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.DeleteOAuthApp(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
