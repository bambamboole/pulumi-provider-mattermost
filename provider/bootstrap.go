package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

const (
	defaultBootstrapTokenDescription = "pulumi"
	generatedPasswordLength          = 40
)

// Bootstrap obtains a personal access token of a system-admin user without
// user interaction. The user is the account everything else is managed with:
// unlike a bot it may create bots, and its token is an ordinary personal
// access token. On a server without accounts the user is signed up as the
// first account, which Mattermost promotes to system admin; on a running
// server it is created or adopted through an existing admin token, or simply
// logged in when its password is known.
type Bootstrap struct{}

type BootstrapArgs struct {
	BaseURL          string       `pulumi:"baseUrl,optional"`
	Username         string       `pulumi:"username" provider:"replaceOnChanges"`
	Email            string       `pulumi:"email"`
	Password         string       `pulumi:"password,optional" provider:"secret"`
	AdminToken       string       `pulumi:"adminToken,optional" provider:"secret"`
	Roles            []SystemRole `pulumi:"roles,optional"`
	TokenDescription string       `pulumi:"tokenDescription,optional"`
}

type BootstrapState struct {
	BootstrapArgs
	UserID            string `pulumi:"userId"`
	GeneratedPassword string `pulumi:"generatedPassword,optional" provider:"secret"`
	TokenID           string `pulumi:"tokenId"`
	Token             string `pulumi:"token" provider:"secret"`
	RepairRequired    bool   `pulumi:"repairRequired,optional"`
}

func (r *Bootstrap) Annotate(a infer.Annotator) {
	a.SetToken("index", "Bootstrap")
	a.Describe(&r, "Obtains a personal access token of a system-admin user without user interaction, for use as the token of a second provider instance. On a fresh server the user is signed up as the first account (which Mattermost promotes to system admin). On a running server the user is logged in with its password, or created or adopted through adminToken. Personal access tokens are enabled on the server when they are not. A refresh that finds the token rejected while the user still logs in with its password (for example after a server reset disabled personal access tokens, or after the token was revoked) marks the resource for repair, and the next update enables personal access tokens again and reissues the token when it is gone. The resource authenticates on its own, so its provider does not need a token. The resource ID is the user ID.")
}

func (args *BootstrapArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.BaseURL, "Base URL of the Mattermost instance. Defaults to the provider's base URL.")
	a.Describe(&args.Username, "Username of the admin user to sign up, create, or adopt. Changing it replaces the resource.")
	a.Describe(&args.Email, "Email of the admin user.")
	a.Describe(&args.Password, "Password of the admin user. Generated and kept in state when unset. Adopting an existing user through adminToken sets it.")
	a.Describe(&args.AdminToken, "Token of an existing system admin (personal access or bot token), needed only to create or adopt the user on a server that already has accounts and to recover when the issued token was revoked.")
	a.Describe(&args.Roles, "System roles of the user. Defaults to [\"system_user\", \"system_admin\"].")
	a.Describe(&args.TokenDescription, "Description of the personal access token. Changing it rotates the token. Defaults to \"pulumi\".")
	a.SetDefault(&args.TokenDescription, defaultBootstrapTokenDescription)
}

func (state *BootstrapState) Annotate(a infer.Annotator) {
	a.Describe(&state.UserID, "ID of the admin user.")
	a.Describe(&state.GeneratedPassword, "The generated password when none was configured.")
	a.Describe(&state.TokenID, "ID of the issued personal access token.")
	a.Describe(&state.Token, "The issued personal access token. Use it as the token of a second provider instance.")
	a.Describe(&state.RepairRequired, "True after a refresh found the token rejected while the user could still be reached with its password or adminToken. The next update repairs the token.")
}

func defaultBootstrapRoles() []SystemRole {
	return normalizeRoles([]SystemRole{SystemRoleUser, SystemRoleAdmin})
}

// effectivePassword is the configured password or the one generated on create.
func (state BootstrapState) effectivePassword() string {
	if state.Password != "" {
		return state.Password
	}
	return state.GeneratedPassword
}

func (Bootstrap) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[BootstrapArgs], error) {
	args, failures, err := infer.DefaultCheck[BootstrapArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[BootstrapArgs]{}, err
	}
	if args.TokenDescription == "" {
		args.TokenDescription = defaultBootstrapTokenDescription
	}
	if len(args.Roles) == 0 {
		args.Roles = defaultBootstrapRoles()
	} else {
		args.Roles = normalizeRoles(args.Roles)
	}
	return infer.CheckResponse[BootstrapArgs]{Inputs: args, Failures: failures}, nil
}

func (Bootstrap) Create(ctx context.Context, req infer.CreateRequest[BootstrapArgs]) (infer.CreateResponse[BootstrapState], error) {
	state := BootstrapState{BootstrapArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[BootstrapState]{Output: state}, nil
	}
	if req.Inputs.Password == "" {
		password, err := generatePassword()
		if err != nil {
			return infer.CreateResponse[BootstrapState]{}, err
		}
		state.GeneratedPassword = password
	}
	baseURL := bootstrapBaseURL(ctx, req.Inputs)
	session, userID, err := bootstrapEnsureUser(ctx, baseURL, req.Inputs, state.effectivePassword())
	if err != nil {
		return infer.CreateResponse[BootstrapState]{}, err
	}
	state.UserID = userID

	if err := bootstrapEnsureRoles(ctx, session, userID, req.Inputs.Roles); err != nil {
		return infer.CreateResponse[BootstrapState]{}, err
	}
	if err := bootstrapEnableUserAccessTokens(ctx, session); err != nil {
		return infer.CreateResponse[BootstrapState]{}, err
	}
	token, _, err := session.API.CreateUserAccessToken(ctx, userID, req.Inputs.TokenDescription, 0)
	if err != nil {
		return infer.CreateResponse[BootstrapState]{}, fmt.Errorf("mattermost: creating access token for %q: %w", req.Inputs.Username, err)
	}
	state.TokenID = token.Id
	state.Token = token.Token
	return infer.CreateResponse[BootstrapState]{ID: userID, Output: state}, nil
}

func (Bootstrap) Read(ctx context.Context, req infer.ReadRequest[BootstrapArgs, BootstrapState]) (infer.ReadResponse[BootstrapArgs, BootstrapState], error) {
	state := req.State
	if state.Token == "" {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, nil
	}
	client, err := mm.New(bootstrapBaseURL(ctx, state.BootstrapArgs), state.Token)
	if err != nil {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, err
	}
	me, response, err := client.API.GetMe(ctx, "")
	if isUnauthorized(response) || isNotFound(response) {
		// The token is rejected: disabled with personal access tokens, revoked,
		// or gone with the user. When the user is still reachable, the next
		// update repairs the token; otherwise the resource is recreated.
		me, err = bootstrapReachUser(ctx, bootstrapBaseURL(ctx, state.BootstrapArgs), state)
		if err != nil {
			return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, err
		}
		if me == nil {
			return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, nil
		}
		state.RepairRequired = true
	} else if err != nil {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, err
	} else {
		state.RepairRequired = false
	}
	if me.Id != state.UserID || me.DeleteAt > 0 {
		return infer.ReadResponse[BootstrapArgs, BootstrapState]{}, nil
	}

	inputs := req.Inputs
	if inputs.Username == "" {
		inputs = state.BootstrapArgs
	}
	inputs.Username = me.Username
	inputs.Email = me.Email
	inputs.Roles = parseRoles(me.Roles)
	state.BootstrapArgs = inputs
	return infer.ReadResponse[BootstrapArgs, BootstrapState]{ID: req.ID, Inputs: inputs, State: state}, nil
}

// Diff compares the inputs like the default diff would and also schedules an
// update when a refresh marked the token for repair.
func (Bootstrap) Diff(_ context.Context, req infer.DiffRequest[BootstrapArgs, BootstrapState]) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	old, next := req.State.BootstrapArgs, req.Inputs
	if old.Username != next.Username {
		diff["username"] = p.PropertyDiff{Kind: p.UpdateReplace, InputDiff: true}
	}
	for name, changed := range map[string]bool{
		"baseUrl":          old.BaseURL != next.BaseURL,
		"email":            old.Email != next.Email,
		"password":         old.Password != next.Password,
		"adminToken":       old.AdminToken != next.AdminToken,
		"roles":            !rolesEqual(old.Roles, next.Roles),
		"tokenDescription": old.TokenDescription != next.TokenDescription,
	} {
		if changed {
			diff[name] = p.PropertyDiff{Kind: p.Update, InputDiff: true}
		}
	}
	return infer.DiffResponse{HasChanges: len(diff) > 0 || req.State.RepairRequired, DetailedDiff: diff}, nil
}

func (Bootstrap) Update(ctx context.Context, req infer.UpdateRequest[BootstrapArgs, BootstrapState]) (infer.UpdateResponse[BootstrapState], error) {
	state := BootstrapState{
		BootstrapArgs:     req.Inputs,
		UserID:            req.State.UserID,
		GeneratedPassword: req.State.GeneratedPassword,
		TokenID:           req.State.TokenID,
		Token:             req.State.Token,
	}
	if req.DryRun {
		return infer.UpdateResponse[BootstrapState]{Output: state}, nil
	}
	session, err := bootstrapManagementSession(ctx, req.State)
	if err != nil {
		return infer.UpdateResponse[BootstrapState]{}, err
	}
	if req.State.RepairRequired {
		tokenID, token, err := bootstrapRepairToken(ctx, session, req.State)
		if err != nil {
			return infer.UpdateResponse[BootstrapState]{}, err
		}
		state.TokenID, state.Token = tokenID, token
	}

	if req.Inputs.Email != req.State.Email {
		email := req.Inputs.Email
		if _, _, err := session.API.PatchUser(ctx, state.UserID, &model.UserPatch{Email: &email}); err != nil {
			return infer.UpdateResponse[BootstrapState]{}, fmt.Errorf("mattermost: updating email: %w", err)
		}
	}
	if !rolesEqual(req.State.Roles, req.Inputs.Roles) {
		if _, err := session.API.UpdateUserRoles(ctx, state.UserID, joinRoles(req.Inputs.Roles)); err != nil {
			return infer.UpdateResponse[BootstrapState]{}, err
		}
	}
	if req.Inputs.Password != "" && req.Inputs.Password != req.State.effectivePassword() {
		if _, err := session.API.UpdateUserPassword(ctx, state.UserID, req.State.effectivePassword(), req.Inputs.Password); err != nil {
			return infer.UpdateResponse[BootstrapState]{}, fmt.Errorf("mattermost: updating password: %w", err)
		}
		state.GeneratedPassword = ""
	}
	if req.Inputs.TokenDescription != req.State.TokenDescription {
		if err := bootstrapEnableUserAccessTokens(ctx, session); err != nil {
			return infer.UpdateResponse[BootstrapState]{}, err
		}
		token, _, err := session.API.CreateUserAccessToken(ctx, state.UserID, req.Inputs.TokenDescription, 0)
		if err != nil {
			return infer.UpdateResponse[BootstrapState]{}, fmt.Errorf("mattermost: rotating access token: %w", err)
		}
		previousTokenID := state.TokenID
		state.TokenID = token.Id
		state.Token = token.Token
		if response, err := session.API.RevokeUserAccessToken(ctx, previousTokenID); err != nil && !isNotFound(response) {
			return infer.UpdateResponse[BootstrapState]{}, fmt.Errorf("mattermost: revoking previous access token: %w", err)
		}
	}
	return infer.UpdateResponse[BootstrapState]{Output: state}, nil
}

// bootstrapReachUser looks the user up without the issued token: through a
// password login, or through adminToken. It returns nil when neither works
// or the user is gone.
func bootstrapReachUser(ctx context.Context, baseURL string, state BootstrapState) (*model.User, error) {
	if password := state.effectivePassword(); password != "" {
		anonymous, err := mm.NewAnonymous(baseURL)
		if err != nil {
			return nil, err
		}
		user, response, err := anonymous.API.Login(ctx, state.Username, password)
		if err == nil {
			return user, nil
		}
		if !isUnauthorized(response) && !isNotFound(response) && !isBadRequest(response) {
			return nil, err
		}
	}
	if strings.TrimSpace(state.AdminToken) != "" {
		admin, err := mm.New(baseURL, state.AdminToken)
		if err != nil {
			return nil, err
		}
		user, response, err := admin.API.GetUser(ctx, state.UserID, "")
		if err == nil {
			return user, nil
		}
		if !isUnauthorized(response) && !isNotFound(response) {
			return nil, err
		}
	}
	return nil, nil
}

// bootstrapRepairToken enables personal access tokens again and keeps the
// issued token when the server accepts it afterwards; otherwise it issues a
// new one and revokes what is left of the old.
func bootstrapRepairToken(ctx context.Context, session *mm.Client, state BootstrapState) (string, string, error) {
	if err := bootstrapEnableUserAccessTokens(ctx, session); err != nil {
		return "", "", err
	}
	if state.Token != "" {
		client, err := mm.New(session.BaseURL, state.Token)
		if err == nil {
			if me, _, err := client.API.GetMe(ctx, ""); err == nil && me.Id == state.UserID {
				return state.TokenID, state.Token, nil
			}
		}
	}
	token, _, err := session.API.CreateUserAccessToken(ctx, state.UserID, state.TokenDescription, 0)
	if err != nil {
		return "", "", fmt.Errorf("mattermost: reissuing access token: %w", err)
	}
	if state.TokenID != "" {
		if response, err := session.API.RevokeUserAccessToken(ctx, state.TokenID); err != nil && !isNotFound(response) {
			return "", "", fmt.Errorf("mattermost: revoking previous access token: %w", err)
		}
	}
	return token.Id, token.Token, nil
}

// Delete revokes the access token. The user is kept: deactivating an admin
// account automatically is not worth the lock-out risk.
func (Bootstrap) Delete(ctx context.Context, req infer.DeleteRequest[BootstrapState]) (infer.DeleteResponse, error) {
	if req.State.TokenID == "" {
		return infer.DeleteResponse{}, nil
	}
	session, err := bootstrapManagementSession(ctx, req.State)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	response, err := session.API.RevokeUserAccessToken(ctx, req.State.TokenID)
	if err != nil && !isNotFound(response) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func bootstrapBaseURL(ctx context.Context, args BootstrapArgs) string {
	if strings.TrimSpace(args.BaseURL) != "" {
		return args.BaseURL
	}
	return client(ctx).BaseURL
}

// bootstrapEnsureUser returns an authenticated session that may manage the
// admin user, and the user's ID. With adminToken the user is created or
// adopted (and its password set to the known one); otherwise the user logs in
// with its password, or is signed up as the first account of a fresh server.
func bootstrapEnsureUser(ctx context.Context, baseURL string, args BootstrapArgs, password string) (*mm.Client, string, error) {
	if strings.TrimSpace(args.AdminToken) != "" {
		admin, err := mm.New(baseURL, args.AdminToken)
		if err != nil {
			return nil, "", err
		}
		if _, _, err := admin.API.GetMe(ctx, ""); err != nil {
			return nil, "", fmt.Errorf("mattermost: adminToken was rejected: %w", err)
		}
		user, response, err := admin.API.GetUserByUsername(ctx, args.Username, "")
		switch {
		case err == nil:
			if user.IsBot {
				return nil, "", fmt.Errorf("mattermost: user %q is a bot; choose another username", args.Username)
			}
			if _, err := admin.API.UpdateUserPassword(ctx, user.Id, "", password); err != nil {
				return nil, "", fmt.Errorf("mattermost: setting the password of %q: %w", args.Username, err)
			}
			return admin, user.Id, nil
		case isNotFound(response):
			user, _, err := admin.API.CreateUser(ctx, &model.User{Username: args.Username, Email: args.Email, Password: password})
			if err != nil {
				return nil, "", fmt.Errorf("mattermost: creating user %q: %w", args.Username, err)
			}
			return admin, user.Id, nil
		default:
			return nil, "", err
		}
	}

	anonymous, err := mm.NewAnonymous(baseURL)
	if err != nil {
		return nil, "", err
	}
	user, _, loginErr := anonymous.API.Login(ctx, args.Username, password)
	if loginErr == nil {
		return anonymous, user.Id, nil
	}
	config, _, err := anonymous.API.GetClientConfig(ctx, "")
	if err != nil {
		return nil, "", fmt.Errorf("mattermost: login as %q failed (%w) and the client configuration could not be read: %w", args.Username, loginErr, err)
	}
	if config["NoAccounts"] != "true" {
		return nil, "", fmt.Errorf("mattermost: login as %q failed and the server already has accounts; set the user's password or provide adminToken: %w", args.Username, loginErr)
	}
	if _, _, err := anonymous.API.CreateUser(ctx, &model.User{Username: args.Username, Email: args.Email, Password: password}); err != nil {
		return nil, "", fmt.Errorf("mattermost: signing up the first admin %q: %w", args.Username, err)
	}
	user, _, err = anonymous.API.Login(ctx, args.Username, password)
	if err != nil {
		return nil, "", fmt.Errorf("mattermost: login as the freshly created admin %q: %w", args.Username, err)
	}
	return anonymous, user.Id, nil
}

// bootstrapManagementSession prefers the issued token and falls back to the
// user's password, then to adminToken, when the token no longer works.
func bootstrapManagementSession(ctx context.Context, state BootstrapState) (*mm.Client, error) {
	baseURL := bootstrapBaseURL(ctx, state.BootstrapArgs)
	if state.Token != "" {
		client, err := mm.New(baseURL, state.Token)
		if err == nil {
			if _, _, err := client.API.GetMe(ctx, ""); err == nil {
				return client, nil
			}
		}
	}
	if password := state.effectivePassword(); password != "" {
		anonymous, err := mm.NewAnonymous(baseURL)
		if err != nil {
			return nil, err
		}
		if _, _, err := anonymous.API.Login(ctx, state.Username, password); err == nil {
			return anonymous, nil
		}
	}
	if strings.TrimSpace(state.AdminToken) != "" {
		return mm.New(baseURL, state.AdminToken)
	}
	return nil, fmt.Errorf("mattermost: the token of %q no longer works and neither the password nor adminToken can log in", state.Username)
}

func bootstrapEnsureRoles(ctx context.Context, session *mm.Client, userID string, roles []SystemRole) error {
	user, _, err := session.API.GetUser(ctx, userID, "")
	if err != nil {
		return err
	}
	if rolesEqual(parseRoles(user.Roles), roles) {
		return nil
	}
	if _, err := session.API.UpdateUserRoles(ctx, userID, joinRoles(roles)); err != nil {
		return fmt.Errorf("mattermost: applying roles: %w", err)
	}
	return nil
}

// Personal access tokens are disabled by default; bots are exempt but users
// are not, so the setting is switched on before the first token is issued.
func bootstrapEnableUserAccessTokens(ctx context.Context, session *mm.Client) error {
	config, _, err := session.API.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("mattermost: reading the server configuration: %w", err)
	}
	if config.ServiceSettings.EnableUserAccessTokens != nil && *config.ServiceSettings.EnableUserAccessTokens {
		return nil
	}
	enabled := true
	patch := &model.Config{}
	patch.ServiceSettings.EnableUserAccessTokens = &enabled
	if _, _, err := session.API.PatchConfig(ctx, patch); err != nil {
		return fmt.Errorf("mattermost: enabling personal access tokens: %w", err)
	}
	return nil
}

// generatePassword returns a random password that satisfies every Mattermost
// password policy: it contains lowercase, uppercase, digits and a symbol.
func generatePassword() (string, error) {
	classes := []string{
		"abcdefghijklmnopqrstuvwxyz",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"0123456789",
		"!#$%&*+-=?@^_~",
	}
	all := strings.Join(classes, "")
	password := make([]byte, 0, generatedPasswordLength)
	for _, class := range classes {
		char, err := randomChar(class)
		if err != nil {
			return "", err
		}
		password = append(password, char)
	}
	for len(password) < generatedPasswordLength {
		char, err := randomChar(all)
		if err != nil {
			return "", err
		}
		password = append(password, char)
	}
	for i := len(password) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		password[i], password[j.Int64()] = password[j.Int64()], password[i]
	}
	return string(password), nil
}

func randomChar(alphabet string) (byte, error) {
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
	if err != nil {
		return 0, fmt.Errorf("mattermost: generating password: %w", err)
	}
	return alphabet[index.Int64()], nil
}

func isUnauthorized(response *model.Response) bool {
	return response != nil && response.StatusCode == http.StatusUnauthorized
}

func isBadRequest(response *model.Response) bool {
	return response != nil && response.StatusCode == http.StatusBadRequest
}
