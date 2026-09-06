package provider

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// User manages a Mattermost user account.
type User struct{}

type UserArgs struct {
	Username  string `pulumi:"username"`
	Email     string `pulumi:"email"`
	Password string `pulumi:"password,optional" provider:"secret"`
	FirstName string `pulumi:"firstName,optional"`
	LastName  string `pulumi:"lastName,optional"`
	Nickname  string `pulumi:"nickname,optional"`
}

type UserState struct {
	UserArgs
	ID string `pulumi:"id"`
}

func (r *User) Annotate(a infer.Annotator) {
	a.SetToken("index", "User")
	a.Describe(&r, "A Mattermost user account. The password is only used on create and is retained as a secret input because the Mattermost API never returns it.")
}

func (User) Create(ctx context.Context, req infer.CreateRequest[UserArgs]) (infer.CreateResponse[UserState], error) {
	state := UserState{UserArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[UserState]{Output: state}, nil
	}
	user, _, err := client(ctx).API.CreateUser(ctx, &model.User{
		Username: req.Inputs.Username,
		Email: req.Inputs.Email,
		Password: req.Inputs.Password,
		FirstName: req.Inputs.FirstName,
		LastName: req.Inputs.LastName,
		Nickname: req.Inputs.Nickname,
	})
	if err != nil {
		return infer.CreateResponse[UserState]{}, err
	}
	state.ID = user.Id
	return infer.CreateResponse[UserState]{ID: user.Id, Output: state}, nil
}

func (User) Update(ctx context.Context, req infer.UpdateRequest[UserArgs, UserState]) (infer.UpdateResponse[UserState], error) {
	state := UserState{UserArgs: req.Inputs, ID: req.ID}
	if req.DryRun {
		return infer.UpdateResponse[UserState]{Output: state}, nil
	}
	_, _, err := client(ctx).API.UpdateUser(ctx, &model.User{
		Id: req.ID,
		Username: req.Inputs.Username,
		Email: req.Inputs.Email,
		FirstName: req.Inputs.FirstName,
		LastName: req.Inputs.LastName,
		Nickname: req.Inputs.Nickname,
	})
	if err != nil {
		return infer.UpdateResponse[UserState]{}, err
	}
	return infer.UpdateResponse[UserState]{Output: state}, nil
}

func (User) Read(ctx context.Context, req infer.ReadRequest[UserArgs, UserState]) (infer.ReadResponse[UserArgs, UserState], error) {
	user, response, err := client(ctx).API.GetUser(ctx, req.ID, "")
	if isNotFound(response) {
		return infer.ReadResponse[UserArgs, UserState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[UserArgs, UserState]{}, err
	}
	inputs := UserArgs{
		Username: user.Username,
		Email: user.Email,
		Password: req.Inputs.Password,
		FirstName: user.FirstName,
		LastName: user.LastName,
		Nickname: user.Nickname,
	}
	return infer.ReadResponse[UserArgs, UserState]{ID: req.ID, Inputs: inputs, State: UserState{UserArgs: inputs, ID: req.ID}}, nil
}

func (User) Delete(ctx context.Context, req infer.DeleteRequest[UserState]) (infer.DeleteResponse, error) {
	response, err := client(ctx).API.DeleteUser(ctx, req.ID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
