package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// TeamMember manages a user's membership in a Mattermost team.
type TeamMember struct{}

type TeamMemberArgs struct {
	TeamID string `pulumi:"teamId"`
	UserID string `pulumi:"userId"`
}

type TeamMemberState struct {
	TeamMemberArgs
}

func (r *TeamMember) Annotate(a infer.Annotator) {
	a.SetToken("index", "TeamMember")
	a.Describe(&r, "Membership of a Mattermost user in a team.")
}

func (TeamMember) Create(ctx context.Context, req infer.CreateRequest[TeamMemberArgs]) (infer.CreateResponse[TeamMemberState], error) {
	state := TeamMemberState{TeamMemberArgs: req.Inputs}
	id := membershipID(req.Inputs.TeamID, req.Inputs.UserID)
	if req.DryRun {
		return infer.CreateResponse[TeamMemberState]{ID: id, Output: state}, nil
	}
	_, _, err := client(ctx).API.AddTeamMember(ctx, req.Inputs.TeamID, req.Inputs.UserID)
	if err != nil {
		return infer.CreateResponse[TeamMemberState]{}, err
	}
	return infer.CreateResponse[TeamMemberState]{ID: id, Output: state}, nil
}

func (TeamMember) Read(ctx context.Context, req infer.ReadRequest[TeamMemberArgs, TeamMemberState]) (infer.ReadResponse[TeamMemberArgs, TeamMemberState], error) {
	teamID, userID, err := membershipParts(req.ID)
	if err != nil {
		return infer.ReadResponse[TeamMemberArgs, TeamMemberState]{}, err
	}
	_, response, err := client(ctx).API.GetTeamMember(ctx, teamID, userID, "")
	if isNotFound(response) {
		return infer.ReadResponse[TeamMemberArgs, TeamMemberState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[TeamMemberArgs, TeamMemberState]{}, err
	}
	inputs := TeamMemberArgs{TeamID: teamID, UserID: userID}
	return infer.ReadResponse[TeamMemberArgs, TeamMemberState]{ID: req.ID, Inputs: inputs, State: TeamMemberState{TeamMemberArgs: inputs}}, nil
}

func (TeamMember) Delete(ctx context.Context, req infer.DeleteRequest[TeamMemberState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.RemoveTeamMember(ctx, req.State.TeamID, req.State.UserID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func membershipID(parentID, userID string) string {
	return parentID + ":" + userID
}

func membershipParts(id string) (string, string, error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("mattermost: invalid membership id %q; expected <parentId>:<userId>", id)
	}
	return parts[0], parts[1], nil
}
