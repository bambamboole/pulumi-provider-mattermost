package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// ChannelMember manages a user's membership in a Mattermost channel.
type ChannelMember struct{}

type ChannelMemberArgs struct {
	ChannelID string `pulumi:"channelId" provider:"replaceOnChanges"`
	UserID    string `pulumi:"userId" provider:"replaceOnChanges"`
	// Whether the user is a channel administrator.
	SchemeAdmin bool `pulumi:"schemeAdmin,optional"`
}

type ChannelMemberState struct {
	ChannelMemberArgs
}

func (r *ChannelMember) Annotate(a infer.Annotator) {
	a.SetToken("index", "ChannelMember")
	a.Describe(&r, "Membership of a Mattermost user in a channel.")
}

func (args *ChannelMemberArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SchemeAdmin, "Grant the channel admin role to the member.")
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
	if req.Inputs.SchemeAdmin {
		if err := setChannelSchemeAdmin(ctx, req.Inputs.ChannelID, req.Inputs.UserID, true); err != nil {
			return infer.CreateResponse[ChannelMemberState]{}, err
		}
	}
	return infer.CreateResponse[ChannelMemberState]{ID: id, Output: state}, nil
}

func (ChannelMember) Update(ctx context.Context, req infer.UpdateRequest[ChannelMemberArgs, ChannelMemberState]) (infer.UpdateResponse[ChannelMemberState], error) {
	state := ChannelMemberState{ChannelMemberArgs: req.Inputs}
	if req.DryRun {
		return infer.UpdateResponse[ChannelMemberState]{Output: state}, nil
	}
	if req.Inputs.SchemeAdmin != req.State.SchemeAdmin {
		if err := setChannelSchemeAdmin(ctx, req.Inputs.ChannelID, req.Inputs.UserID, req.Inputs.SchemeAdmin); err != nil {
			return infer.UpdateResponse[ChannelMemberState]{}, err
		}
	}
	return infer.UpdateResponse[ChannelMemberState]{Output: state}, nil
}

func setChannelSchemeAdmin(ctx context.Context, channelID, userID string, admin bool) error {
	_, err := client(ctx).API.UpdateChannelMemberSchemeRoles(ctx, channelID, userID, &model.SchemeRoles{SchemeUser: true, SchemeAdmin: admin})
	return err
}

func (ChannelMember) Read(ctx context.Context, req infer.ReadRequest[ChannelMemberArgs, ChannelMemberState]) (infer.ReadResponse[ChannelMemberArgs, ChannelMemberState], error) {
	channelID, userID, err := membershipParts(req.ID)
	if err != nil {
		return infer.ReadResponse[ChannelMemberArgs, ChannelMemberState]{}, err
	}
	member, response, err := client(ctx).API.GetChannelMember(ctx, channelID, userID, "")
	if isNotFound(response) {
		return infer.ReadResponse[ChannelMemberArgs, ChannelMemberState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[ChannelMemberArgs, ChannelMemberState]{}, err
	}
	inputs := ChannelMemberArgs{ChannelID: channelID, UserID: userID, SchemeAdmin: member.SchemeAdmin}
	return infer.ReadResponse[ChannelMemberArgs, ChannelMemberState]{ID: req.ID, Inputs: inputs, State: ChannelMemberState{ChannelMemberArgs: inputs}}, nil
}

func (ChannelMember) Delete(ctx context.Context, req infer.DeleteRequest[ChannelMemberState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.RemoveUserFromChannel(ctx, req.State.ChannelID, req.State.UserID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
