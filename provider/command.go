package provider

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Command is a custom slash command of a team. Mattermost issues a token per
// command that the receiving endpoint verifies; it is kept in state as a
// secret.
type Command struct{}

type CommandArgs struct {
	TeamID           string `pulumi:"teamId"`
	Trigger          string `pulumi:"trigger"`
	URL              string `pulumi:"url"`
	Method           string `pulumi:"method,optional"`
	DisplayName      string `pulumi:"displayName,optional"`
	Description      string `pulumi:"description,optional"`
	AutoComplete     bool   `pulumi:"autoComplete,optional"`
	AutoCompleteDesc string `pulumi:"autoCompleteDesc,optional"`
	AutoCompleteHint string `pulumi:"autoCompleteHint,optional"`
	Username         string `pulumi:"username,optional"`
	IconURL          string `pulumi:"iconUrl,optional"`
}

type CommandState struct {
	CommandArgs
	CreatorID string `pulumi:"creatorId"`
	Token     string `pulumi:"token" provider:"secret"`
}

func (r *Command) Annotate(a infer.Annotator) {
	a.SetToken("index", "Command")
	a.Describe(&r, "A custom slash command of a team. Mattermost sends the command's token with every request to the URL, so the endpoint can verify the caller; the token is kept in state as a secret. The provider's account needs manage_slash_commands on the team. The resource ID is the command ID.")
}

func (args *CommandArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.TeamID, "ID of the team the command belongs to.")
	a.Describe(&args.Trigger, "The word after the slash that invokes the command, without the slash. Mattermost stores it in lower case.")
	a.Describe(&args.URL, "The HTTP or HTTPS URL Mattermost calls when the command is executed.")
	a.Describe(&args.Method, "How Mattermost calls the URL: `P` for POST (form-encoded body) or `G` for GET (query string). Defaults to `P`.")
	a.SetDefault(&args.Method, model.CommandMethodPost)
	a.Describe(&args.DisplayName, "Name shown in the integrations list (at most 64 characters).")
	a.Describe(&args.Description, "Description shown in the integrations list (at most 128 characters).")
	a.Describe(&args.AutoComplete, "Whether the command appears in the autocomplete list while typing a slash.")
	a.Describe(&args.AutoCompleteDesc, "Short description shown in the autocomplete list.")
	a.Describe(&args.AutoCompleteHint, "Argument hint shown in the autocomplete list, e.g. `[text]`.")
	a.Describe(&args.Username, "Username the command's responses are posted as; the caller's username when unset.")
	a.Describe(&args.IconURL, "Profile picture the command's responses are posted with.")
}

func (state *CommandState) Annotate(a infer.Annotator) {
	a.Describe(&state.CreatorID, "ID of the user who created the command.")
	a.Describe(&state.Token, "The token Mattermost sends with every request to the URL.")
}

func (Command) Create(ctx context.Context, req infer.CreateRequest[CommandArgs]) (infer.CreateResponse[CommandState], error) {
	state := CommandState{CommandArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[CommandState]{Output: state}, nil
	}
	command, _, err := client(ctx).API.CreateCommand(ctx, commandModel(req.Inputs, ""))
	if err != nil {
		return infer.CreateResponse[CommandState]{}, fmt.Errorf("mattermost: creating command /%s: %w", req.Inputs.Trigger, err)
	}
	state.CreatorID = command.CreatorId
	state.Token = command.Token
	return infer.CreateResponse[CommandState]{ID: command.Id, Output: state}, nil
}

// Update keeps the token: Mattermost only rotates it through the dedicated
// regenerate endpoint, which this resource does not call.
func (Command) Update(ctx context.Context, req infer.UpdateRequest[CommandArgs, CommandState]) (infer.UpdateResponse[CommandState], error) {
	state := CommandState{CommandArgs: req.Inputs, CreatorID: req.State.CreatorID, Token: req.State.Token}
	if req.DryRun {
		return infer.UpdateResponse[CommandState]{Output: state}, nil
	}
	command, _, err := client(ctx).API.UpdateCommand(ctx, commandModel(req.Inputs, req.ID))
	if err != nil {
		return infer.UpdateResponse[CommandState]{}, fmt.Errorf("mattermost: updating command /%s: %w", req.Inputs.Trigger, err)
	}
	state.CreatorID = firstNonEmpty(command.CreatorId, req.State.CreatorID)
	state.Token = firstNonEmpty(command.Token, req.State.Token)
	return infer.UpdateResponse[CommandState]{Output: state}, nil
}

// Read supports import. Mattermost sanitizes the token, method, URL, username
// and icon for accounts that may not manage the command, in which case the
// values already in state are kept.
func (Command) Read(ctx context.Context, req infer.ReadRequest[CommandArgs, CommandState]) (infer.ReadResponse[CommandArgs, CommandState], error) {
	command, response, err := client(ctx).API.GetCommandById(ctx, req.ID)
	if isNotFound(response) {
		return infer.ReadResponse[CommandArgs, CommandState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[CommandArgs, CommandState]{}, err
	}
	inputs := CommandArgs{
		TeamID:           command.TeamId,
		Trigger:          command.Trigger,
		URL:              firstNonEmpty(command.URL, req.Inputs.URL),
		Method:           firstNonEmpty(command.Method, req.Inputs.Method),
		DisplayName:      command.DisplayName,
		Description:      command.Description,
		AutoComplete:     command.AutoComplete,
		AutoCompleteDesc: command.AutoCompleteDesc,
		AutoCompleteHint: command.AutoCompleteHint,
		Username:         firstNonEmpty(command.Username, req.Inputs.Username),
		IconURL:          firstNonEmpty(command.IconURL, req.Inputs.IconURL),
	}
	state := CommandState{
		CommandArgs: inputs,
		CreatorID:   firstNonEmpty(command.CreatorId, req.State.CreatorID),
		Token:       firstNonEmpty(command.Token, req.State.Token),
	}
	return infer.ReadResponse[CommandArgs, CommandState]{ID: command.Id, Inputs: inputs, State: state}, nil
}

func (Command) Delete(ctx context.Context, req infer.DeleteRequest[CommandState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.DeleteCommand(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func commandModel(args CommandArgs, id string) *model.Command {
	return &model.Command{
		Id:               id,
		TeamId:           args.TeamID,
		Trigger:          args.Trigger,
		URL:              args.URL,
		Method:           firstNonEmpty(args.Method, model.CommandMethodPost),
		DisplayName:      args.DisplayName,
		Description:      args.Description,
		AutoComplete:     args.AutoComplete,
		AutoCompleteDesc: args.AutoCompleteDesc,
		AutoCompleteHint: args.AutoCompleteHint,
		Username:         args.Username,
		IconURL:          args.IconURL,
	}
}
