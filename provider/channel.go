package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Channel manages a public or private Mattermost channel.
type Channel struct{}

type ChannelArgs struct {
	TeamID      string `pulumi:"teamId" provider:"replaceOnChanges"`
	Name        string `pulumi:"name"`
	DisplayName string `pulumi:"displayName"`
	Purpose     string `pulumi:"purpose,optional"`
	Header      string `pulumi:"header,optional"`
	Type        string `pulumi:"type,optional"`
}

type ChannelState struct {
	ChannelArgs
}

func (r *Channel) Annotate(a infer.Annotator) {
	a.SetToken("index", "Channel")
	a.Describe(&r, "A Mattermost public or private channel. Deleting the resource archives the channel; it is not removed permanently.")
}

func (Channel) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ChannelArgs], error) {
	args, failures, err := infer.DefaultCheck[ChannelArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[ChannelArgs]{}, err
	}
	if args.Type == "" {
		args.Type = string(model.ChannelTypeOpen)
	}
	if args.Type != string(model.ChannelTypeOpen) && args.Type != string(model.ChannelTypePrivate) {
		failures = append(failures, p.CheckFailure{Property: "type", Reason: `must be "O" (public) or "P" (private)`})
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
	return infer.CreateResponse[ChannelState]{ID: channel.Id, Output: state}, nil
}

func (Channel) Update(ctx context.Context, req infer.UpdateRequest[ChannelArgs, ChannelState]) (infer.UpdateResponse[ChannelState], error) {
	state := ChannelState{ChannelArgs: req.Inputs}
	if req.DryRun {
		return infer.UpdateResponse[ChannelState]{Output: state}, nil
	}
	api := client(ctx).API
	patch := &model.ChannelPatch{
		Name:        &req.Inputs.Name,
		DisplayName: &req.Inputs.DisplayName,
		Purpose:     &req.Inputs.Purpose,
		Header:      &req.Inputs.Header,
	}
	if _, _, err := api.PatchChannel(ctx, req.ID, patch); err != nil {
		return infer.UpdateResponse[ChannelState]{}, err
	}
	// The update endpoints reject a changed type; public/private conversion has its own endpoint.
	if req.Inputs.Type != req.State.Type {
		if _, _, err := api.UpdateChannelPrivacy(ctx, req.ID, model.ChannelType(req.Inputs.Type)); err != nil {
			return infer.UpdateResponse[ChannelState]{}, err
		}
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
	// Delete only archives the channel; the API keeps returning it. Treat an
	// archived channel as gone so Pulumi recreates it instead of adopting it.
	if channel.DeleteAt > 0 {
		return infer.ReadResponse[ChannelArgs, ChannelState]{}, nil
	}
	inputs := ChannelArgs{TeamID: channel.TeamId, Name: channel.Name, DisplayName: channel.DisplayName, Purpose: channel.Purpose, Header: channel.Header, Type: string(channel.Type)}
	return infer.ReadResponse[ChannelArgs, ChannelState]{ID: req.ID, Inputs: inputs, State: ChannelState{ChannelArgs: inputs}}, nil
}

func (Channel) Delete(ctx context.Context, req infer.DeleteRequest[ChannelState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.DeleteChannel(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
