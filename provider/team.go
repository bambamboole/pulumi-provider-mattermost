package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
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
	ID string `pulumi:"id"`
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
		args.Type = "O"
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
	state.ID = team.Id
	return infer.CreateResponse[TeamState]{ID: team.Id, Output: state}, nil
}

func (Team) Update(ctx context.Context, req infer.UpdateRequest[TeamArgs, TeamState]) (infer.UpdateResponse[TeamState], error) {
	state := TeamState{TeamArgs: req.Inputs, ID: req.ID}
	if req.DryRun {
		return infer.UpdateResponse[TeamState]{Output: state}, nil
	}
	_, _, err := client(ctx).API.UpdateTeam(ctx, &model.Team{Id: req.ID, Name: req.Inputs.Name, DisplayName: req.Inputs.DisplayName, Description: req.Inputs.Description, Type: req.Inputs.Type})
	if err != nil {
		return infer.UpdateResponse[TeamState]{}, err
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
	return infer.ReadResponse[TeamArgs, TeamState]{ID: req.ID, Inputs: inputs, State: TeamState{TeamArgs: inputs, ID: req.ID}}, nil
}

func (Team) Delete(ctx context.Context, req infer.DeleteRequest[TeamState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.PermanentDeleteTeam(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
