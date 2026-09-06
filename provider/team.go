package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Team manages a Mattermost team.
type Team struct{}

type TeamArgs struct {
	Name        string `pulumi:"name"`
	DisplayName string `pulumi:"displayName"`
	Description string `pulumi:"description,optional"`
	Type        string `pulumi:"type,optional"`
}

type TeamState struct {
	TeamArgs
}

func (r *Team) Annotate(a infer.Annotator) {
	a.SetToken("index", "Team")
	a.Describe(&r, "A Mattermost team.")
}

func (Team) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[TeamArgs], error) {
	args, failures, err := infer.DefaultCheck[TeamArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[TeamArgs]{}, err
	}
	if args.Type == "" {
		args.Type = model.TeamOpen
	}
	if args.Type != model.TeamOpen && args.Type != model.TeamInvite {
		failures = append(failures, p.CheckFailure{Property: "type", Reason: `must be "O" (open) or "I" (invite only)`})
	}
	return infer.CheckResponse[TeamArgs]{Inputs: args, Failures: failures}, nil
}

func (Team) Create(ctx context.Context, req infer.CreateRequest[TeamArgs]) (infer.CreateResponse[TeamState], error) {
	state := TeamState{TeamArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[TeamState]{Output: state}, nil
	}
	team, _, err := client(ctx).API.CreateTeam(ctx, &model.Team{Name: req.Inputs.Name, DisplayName: req.Inputs.DisplayName, Description: req.Inputs.Description, Type: req.Inputs.Type})
	if err != nil {
		return infer.CreateResponse[TeamState]{}, err
	}
	return infer.CreateResponse[TeamState]{ID: team.Id, Output: state}, nil
}

func (Team) Update(ctx context.Context, req infer.UpdateRequest[TeamArgs, TeamState]) (infer.UpdateResponse[TeamState], error) {
	state := TeamState{TeamArgs: req.Inputs}
	if req.DryRun {
		return infer.UpdateResponse[TeamState]{Output: state}, nil
	}
	api := client(ctx).API
	if req.Inputs.Name != req.State.Name {
		// The team name (URL slug) is not part of TeamPatch. UpdateTeam overwrites
		// every settable field, so send the current team with our changes applied
		// instead of a sparse struct that would reset unmanaged settings.
		team, _, err := api.GetTeam(ctx, req.ID, "")
		if err != nil {
			return infer.UpdateResponse[TeamState]{}, err
		}
		team.Name = req.Inputs.Name
		team.DisplayName = req.Inputs.DisplayName
		team.Description = req.Inputs.Description
		if _, _, err := api.UpdateTeam(ctx, team); err != nil {
			return infer.UpdateResponse[TeamState]{}, err
		}
	} else {
		patch := &model.TeamPatch{DisplayName: &req.Inputs.DisplayName, Description: &req.Inputs.Description}
		if _, _, err := api.PatchTeam(ctx, req.ID, patch); err != nil {
			return infer.UpdateResponse[TeamState]{}, err
		}
	}
	// UpdateTeam ignores the type field; privacy changes go through their own endpoint.
	if req.Inputs.Type != req.State.Type {
		if _, _, err := api.UpdateTeamPrivacy(ctx, req.ID, req.Inputs.Type); err != nil {
			return infer.UpdateResponse[TeamState]{}, err
		}
	}
	return infer.UpdateResponse[TeamState]{Output: state}, nil
}

func (Team) Read(ctx context.Context, req infer.ReadRequest[TeamArgs, TeamState]) (infer.ReadResponse[TeamArgs, TeamState], error) {
	team, response, err := client(ctx).API.GetTeam(ctx, req.ID, "")
	if isNotFound(response) {
		return infer.ReadResponse[TeamArgs, TeamState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[TeamArgs, TeamState]{}, err
	}
	inputs := TeamArgs{Name: team.Name, DisplayName: team.DisplayName, Description: team.Description, Type: team.Type}
	return infer.ReadResponse[TeamArgs, TeamState]{ID: req.ID, Inputs: inputs, State: TeamState{TeamArgs: inputs}}, nil
}

func (Team) Delete(ctx context.Context, req infer.DeleteRequest[TeamState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.PermanentDeleteTeam(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
