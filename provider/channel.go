package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Channel manages a public or private Mattermost channel.
type Channel struct{}

type ChannelArgs struct {
	TeamID      string `pulumi:"teamId"`
	Name        string `pulumi:"name"`
	DisplayName string `pulumi:"displayName"`
	Purpose     string `pulumi:"purpose,optional"`
	Header      string `pulumi:"header,optional"`
	Type        string `pulumi:"type,optional"`
}

type ChannelState struct {
	ChannelArgs
	ID string `pulumi:"id"`
}

func (r *Channel) Annotate(a infer.Annotator) {
	a.SetToken("index", "Channel")
	a.Describe(&r, "A Mattermost public or private channel.")
}

func (Channel) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ChannelArgs], error) {
	args, failures, err := infer.DefaultCheck[ChannelArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[ChannelArgs]{}, err
	}
	if args.Type == "" {
		args.Type = "O"
	}
	return infer.CheckResponse[ChannelArgs]{Inputs: args, Failures: failures}, nil
}

func (Channel) Create(ctx context.Context, req infer.CreateRequest[ChannelArgs]) (infer.CreateResponse[ChannelState], error) {
	state := ChannelState{ChannelArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[ChannelState]{Output: state}, nil
	}
	channel, _, err := client(ctx).API.CreateChannel(ctx, &model.Channel{TeamId: req.Inputs.TeamID, Name: req.Inputs.Name, DisplayName: req.Inputs.DisplayName, Purpose: req.Inputs.Purpose, Header: req.Inputs.Header, Type: model.ChannelType(req.Inputs.Type)})
	if err != nil {
		return infer.CreateResponse[ChannelState]{}, err
	}
	state.ID = channel.Id
	return infer.CreateResponse[ChannelState]{ID: channel.Id, Output: state}, nil
}

func (Channel) Update(ctx context.Context, req infer.UpdateRequest[ChannelArgs, ChannelState]) (infer.UpdateResponse[ChannelState], error) {
	state := ChannelState{ChannelArgs: req.Inputs, ID: req.ID}
	if req.DryRun {
		return infer.UpdateResponse[ChannelState]{Output: state}, nil
	}
	_, _, err := client(ctx).API.UpdateChannel(ctx, &model.Channel{Id: req.ID, TeamId: req.Inputs.TeamID, Name: req.Inputs.Name, DisplayName: req.Inputs.DisplayName, Purpose: req.Inputs.Purpose, Header: req.Inputs.Header, Type: model.ChannelType(req.Inputs.Type)})
	if err != nil {
		return infer.UpdateResponse[ChannelState]{}, err
	}
	return infer.UpdateResponse[ChannelState]{Output: state}, nil
}

func (Channel) Read(ctx context.Context, req infer.ReadRequest[ChannelArgs, ChannelState]) (infer.ReadResponse[ChannelArgs, ChannelState], error) {
	channel, response, err := client(ctx).API.GetChannel(ctx, req.ID)
	if isNotFound(response) {
		return infer.ReadResponse[ChannelArgs, ChannelState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[ChannelArgs, ChannelState]{}, err
	}
	inputs := ChannelArgs{TeamID: channel.TeamId, Name: channel.Name, DisplayName: channel.DisplayName, Purpose: channel.Purpose, Header: channel.Header, Type: string(channel.Type)}
	return infer.ReadResponse[ChannelArgs, ChannelState]{ID: req.ID, Inputs: inputs, State: ChannelState{ChannelArgs: inputs, ID: req.ID}}, nil
}

func (Channel) Delete(ctx context.Context, req infer.DeleteRequest[ChannelState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.DeleteChannel(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
