package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// ChannelMember manages a user's membership in a Mattermost channel.
type ChannelMember struct{}

type ChannelMemberArgs struct {
	ChannelID string `pulumi:"channelId"`
	UserID    string `pulumi:"userId"`
}

type ChannelMemberState struct {
	ChannelMemberArgs
}

func (r *ChannelMember) Annotate(a infer.Annotator) {
	a.SetToken("index", "ChannelMember")
	a.Describe(&r, "Membership of a Mattermost user in a channel.")
}

func (ChannelMember) Create(ctx context.Context, req infer.CreateRequest[ChannelMemberArgs]) (infer.CreateResponse[ChannelMemberState], error) {
	state := ChannelMemberState{ChannelMemberArgs: req.Inputs}
	id := membershipID(req.Inputs.ChannelID, req.Inputs.UserID)
	if req.DryRun {
		return infer.CreateResponse[ChannelMemberState]{ID: id, Output: state}, nil
	}
	_, _, err := client(ctx).API.AddChannelMember(ctx, req.Inputs.ChannelID, req.Inputs.UserID)
	if err != nil {
		return infer.CreateResponse[ChannelMemberState]{}, err
	}
	return infer.CreateResponse[ChannelMemberState]{ID: id, Output: state}, nil
}

func (ChannelMember) Read(ctx context.Context, req infer.ReadRequest[ChannelMemberArgs, ChannelMemberState]) (infer.ReadResponse[ChannelMemberArgs, ChannelMemberState], error) {
	channelID, userID, err := membershipParts(req.ID)
	if err != nil {
		return infer.ReadResponse[ChannelMemberArgs, ChannelMemberState]{}, err
	}
	_, response, err := client(ctx).API.GetChannelMember(ctx, channelID, userID, "")
	if isNotFound(response) {
		return infer.ReadResponse[ChannelMemberArgs, ChannelMemberState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[ChannelMemberArgs, ChannelMemberState]{}, err
	}
	inputs := ChannelMemberArgs{ChannelID: channelID, UserID: userID}
	return infer.ReadResponse[ChannelMemberArgs, ChannelMemberState]{ID: req.ID, Inputs: inputs, State: ChannelMemberState{ChannelMemberArgs: inputs}}, nil
}

func (ChannelMember) Delete(ctx context.Context, req infer.DeleteRequest[ChannelMemberState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.RemoveUserFromChannel(ctx, req.State.ChannelID, req.State.UserID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
