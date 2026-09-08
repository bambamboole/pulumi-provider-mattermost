package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

type fakeUser struct {
	id       string
	email    string
	password string
	roles    string
	isBot    bool
}

// fakeMattermost is a minimal in-memory Mattermost API for the bootstrap flow.
type fakeMattermost struct {
	t *testing.T

	mu           sync.Mutex
	users        map[string]*fakeUser // username -> user
	tokens       map[string]string    // token value -> user id
	personal     map[string]bool      // token value -> issued as a personal access token
	tokensOn     bool                 // ServiceSettings.EnableUserAccessTokens
	nextToken    int
	calls        []string
	failToken    bool // reject every issued access token
	failPassword bool // reject every password login
}

func newFakeMattermost(t *testing.T) *fakeMattermost {
	return &fakeMattermost{t: t, users: map[string]*fakeUser{}, tokens: map[string]string{}, personal: map[string]bool{}}
}

// addHumanAdmin adds an existing system admin and returns one of its tokens.
func (f *fakeMattermost) addHumanAdmin() string {
	f.users["human"] = &fakeUser{id: "user-human", email: "human@example.com", password: "human-secret", roles: "system_admin system_user"}
	return f.issueToken("user-human")
}

func (f *fakeMattermost) byID(id string) (string, *fakeUser) {
	for username, user := range f.users {
		if user.id == id {
			return username, user
		}
	}
	return "", nil
}

func (f *fakeMattermost) authorizedUser(r *http.Request) (string, bool) {
	parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	userID, ok := f.tokens[parts[1]]
	return userID, ok
}

func (f *fakeMattermost) issueToken(userID string) string {
	f.nextToken++
	token := "token-" + strings.Repeat("x", f.nextToken)
	f.tokens[token] = userID
	return token
}

func (f *fakeMattermost) userJSON(username string, user *fakeUser) map[string]any {
	return map[string]any{"id": user.id, "username": username, "email": user.email, "roles": user.roles, "is_bot": user.isBot}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, id string) {
	writeJSON(w, status, map[string]any{"id": id, "message": id, "status_code": status})
}

func (f *fakeMattermost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	path := strings.TrimPrefix(r.URL.Path, "/api/v4")
	userID, authorized := f.authorizedUser(r)
	if authorized && f.failToken && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer token-") {
		if _, user := f.byID(userID); user != nil && !strings.HasPrefix(user.id, "user-human") {
			authorized = false
		}
	}
	// Mattermost rejects personal access tokens while the setting is off.
	if parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2); authorized && !f.tokensOn && len(parts) == 2 && f.personal[parts[1]] {
		authorized = false
	}

	switch {
	case r.Method == http.MethodGet && path == "/config/client":
		noAccounts := "false"
		if len(f.users) == 0 {
			noAccounts = "true"
		}
		writeJSON(w, http.StatusOK, map[string]string{"NoAccounts": noAccounts})
	case r.Method == http.MethodPost && path == "/users/login":
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		user, ok := f.users[body["login_id"]]
		if !ok || f.failPassword || user.password != body["password"] {
			writeError(w, http.StatusUnauthorized, "api.user.login.invalid_credentials")
			return
		}
		w.Header().Set("Token", f.issueToken(user.id))
		writeJSON(w, http.StatusOK, f.userJSON(body["login_id"], user))
	case r.Method == http.MethodPost && path == "/users":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		username := body["username"].(string)
		user := &fakeUser{id: "user-" + username, email: body["email"].(string), password: body["password"].(string), roles: "system_user"}
		switch {
		case len(f.users) == 0:
			user.roles = "system_admin system_user" // the first account becomes system admin
		case !authorized:
			writeError(w, http.StatusForbidden, "api.user.create_user.no_open_server")
			return
		}
		f.users[username] = user
		writeJSON(w, http.StatusCreated, f.userJSON(username, user))
	case !authorized:
		writeError(w, http.StatusUnauthorized, "api.context.session_expired.app_error")
	case r.Method == http.MethodGet && path == "/users/me":
		username, user := f.byID(userID)
		writeJSON(w, http.StatusOK, f.userJSON(username, user))
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/users/username/"):
		username := strings.TrimPrefix(path, "/users/username/")
		user, ok := f.users[username]
		if !ok {
			writeError(w, http.StatusNotFound, "app.user.get_by_username.app_error")
			return
		}
		writeJSON(w, http.StatusOK, f.userJSON(username, user))
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/users/") && !strings.Contains(strings.TrimPrefix(path, "/users/"), "/"):
		username, user := f.byID(strings.TrimPrefix(path, "/users/"))
		if user == nil {
			writeError(w, http.StatusNotFound, "app.user.missing_account.const")
			return
		}
		writeJSON(w, http.StatusOK, f.userJSON(username, user))
	case r.Method == http.MethodPut && strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/patch"):
		var patch map[string]any
		_ = json.NewDecoder(r.Body).Decode(&patch)
		username, user := f.byID(strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/patch"))
		if email, ok := patch["email"].(string); ok {
			user.email = email
		}
		writeJSON(w, http.StatusOK, f.userJSON(username, user))
	case r.Method == http.MethodPut && strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/roles"):
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, user := f.byID(strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/roles"))
		user.roles = body["roles"]
		writeJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	case r.Method == http.MethodPut && strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/password"):
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		targetID := strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/password")
		_, user := f.byID(targetID)
		if targetID == userID && body["current_password"] != user.password {
			writeError(w, http.StatusBadRequest, "api.user.update_password.incorrect.app_error")
			return
		}
		user.password = body["new_password"]
		writeJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/tokens") && path != "/users/tokens":
		if !f.tokensOn {
			writeError(w, http.StatusNotImplemented, "api.user.create_user_access_token.disabled.app_error")
			return
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/tokens")
		token := f.issueToken(id)
		f.personal[token] = true
		writeJSON(w, http.StatusOK, map[string]any{"id": "token-id-" + token, "token": token, "user_id": id, "description": body["description"], "is_active": true})
	case r.Method == http.MethodPost && path == "/users/tokens/revoke":
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		token := strings.TrimPrefix(body["token_id"], "token-id-")
		if _, ok := f.tokens[token]; !ok {
			writeError(w, http.StatusNotFound, "app.user_access_token.get.app_error")
			return
		}
		delete(f.tokens, token)
		writeJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	case r.Method == http.MethodGet && path == "/config":
		writeJSON(w, http.StatusOK, map[string]any{"ServiceSettings": map[string]any{"EnableUserAccessTokens": f.tokensOn}})
	case r.Method == http.MethodPut && path == "/config/patch":
		var body map[string]map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if enabled, ok := body["ServiceSettings"]["EnableUserAccessTokens"].(bool); ok {
			f.tokensOn = enabled
		}
		writeJSON(w, http.StatusOK, map[string]any{"ServiceSettings": map[string]any{"EnableUserAccessTokens": f.tokensOn}})
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		writeError(w, http.StatusNotFound, "unexpected")
	}
}

func (f *fakeMattermost) called(call string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c == call {
			return true
		}
	}
	return false
}

func (f *fakeMattermost) resetCalls() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}

func bootstrapArgs(baseURL string) BootstrapArgs {
	return BootstrapArgs{
		BaseURL:          baseURL,
		Username:         "infrastructure",
		Email:            "infrastructure@example.com",
		Roles:            defaultBootstrapRoles(),
		TokenDescription: "pulumi",
	}
}

func create(t *testing.T, args BootstrapArgs) BootstrapState {
	t.Helper()
	response, err := (Bootstrap{}).Create(context.Background(), infer.CreateRequest[BootstrapArgs]{Inputs: args})
	if err != nil {
		t.Fatal(err)
	}
	return response.Output
}

func TestBootstrapCreateSignsUpFirstAdminOnFreshServer(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()

	state := create(t, bootstrapArgs(server.URL))

	user := fake.users["infrastructure"]
	if user == nil || state.UserID != "user-infrastructure" || user.email != "infrastructure@example.com" {
		t.Fatalf("expected the user to be signed up, state %#v", state)
	}
	if len(state.GeneratedPassword) != generatedPasswordLength || user.password != state.GeneratedPassword {
		t.Fatalf("expected a generated password to be set on the account, got %q", state.GeneratedPassword)
	}
	if !fake.tokensOn || !fake.called("PUT /api/v4/config/patch") {
		t.Fatal("expected personal access tokens to be enabled")
	}
	if fake.tokens[state.Token] != state.UserID || state.TokenID == "" {
		t.Fatalf("expected a token of the user, got %#v", state)
	}
	if fake.called("PUT /api/v4/users/user-infrastructure/roles") {
		t.Fatal("the first account already is system admin; roles must not be rewritten")
	}
}

func TestBootstrapCreateCreatesUserWithAdminToken(t *testing.T) {
	fake := newFakeMattermost(t)
	adminToken := fake.addHumanAdmin()
	fake.tokensOn = true
	server := httptest.NewServer(fake)
	defer server.Close()

	args := bootstrapArgs(server.URL)
	args.AdminToken = adminToken
	state := create(t, args)

	if fake.called("POST /api/v4/users/login") || fake.called("GET /api/v4/config/client") {
		t.Fatalf("adminToken must be used directly, calls: %v", fake.calls)
	}
	user := fake.users["infrastructure"]
	if user == nil || user.password != state.GeneratedPassword || user.roles != "system_admin system_user" {
		t.Fatalf("unexpected user %#v", user)
	}
	if fake.called("PUT /api/v4/config/patch") {
		t.Fatal("tokens were already enabled")
	}
	if fake.tokens[state.Token] != "user-infrastructure" {
		t.Fatal("token must belong to the new user, not the admin")
	}
}

func TestBootstrapCreateAdoptsExistingUserWithAdminToken(t *testing.T) {
	fake := newFakeMattermost(t)
	adminToken := fake.addHumanAdmin()
	fake.users["infrastructure"] = &fakeUser{id: "user-infrastructure", email: "old@example.com", password: "forgotten", roles: "system_user"}
	server := httptest.NewServer(fake)
	defer server.Close()

	args := bootstrapArgs(server.URL)
	args.AdminToken = adminToken
	args.Password = "Configured-1!"
	state := create(t, args)

	if fake.called("POST /api/v4/users") {
		t.Fatal("existing user must be adopted, not created")
	}
	user := fake.users["infrastructure"]
	if user.password != "Configured-1!" || state.GeneratedPassword != "" {
		t.Fatalf("expected the configured password to be set, got %q", user.password)
	}
	if user.roles != "system_admin system_user" {
		t.Fatalf("expected roles to be applied, got %q", user.roles)
	}
	if fake.tokens[state.Token] != "user-infrastructure" || !fake.tokensOn {
		t.Fatalf("expected an enabled user token, got %#v", state)
	}
}

func TestBootstrapCreateRefusesBotUsername(t *testing.T) {
	fake := newFakeMattermost(t)
	adminToken := fake.addHumanAdmin()
	fake.users["pulumi"] = &fakeUser{id: "bot-pulumi", roles: "system_user", isBot: true}
	server := httptest.NewServer(fake)
	defer server.Close()

	args := bootstrapArgs(server.URL)
	args.AdminToken = adminToken
	args.Username = "pulumi"
	_, err := (Bootstrap{}).Create(context.Background(), infer.CreateRequest[BootstrapArgs]{Inputs: args})
	if err == nil || !strings.Contains(err.Error(), "is a bot") {
		t.Fatalf("expected a bot error, got %v", err)
	}
}

func TestBootstrapCreateLogsInWithKnownPassword(t *testing.T) {
	fake := newFakeMattermost(t)
	fake.addHumanAdmin()
	fake.users["infrastructure"] = &fakeUser{id: "user-infrastructure", email: "infrastructure@example.com", password: "Known-1!", roles: "system_admin system_user"}
	server := httptest.NewServer(fake)
	defer server.Close()

	args := bootstrapArgs(server.URL)
	args.Password = "Known-1!"
	state := create(t, args)

	if !fake.called("POST /api/v4/users/login") || fake.called("POST /api/v4/users") {
		t.Fatalf("expected a password login, calls: %v", fake.calls)
	}
	if fake.tokens[state.Token] != "user-infrastructure" || !fake.tokensOn {
		t.Fatalf("expected the user's token, got %#v", state)
	}
}

func TestBootstrapCreateFailsWithoutCredentialsOnExistingServer(t *testing.T) {
	fake := newFakeMattermost(t)
	fake.addHumanAdmin()
	server := httptest.NewServer(fake)
	defer server.Close()

	_, err := (Bootstrap{}).Create(context.Background(), infer.CreateRequest[BootstrapArgs]{Inputs: bootstrapArgs(server.URL)})
	if err == nil || !strings.Contains(err.Error(), "already has accounts") {
		t.Fatalf("expected a credentials error, got %v", err)
	}
	if fake.called("POST /api/v4/users") {
		t.Fatal("must not sign up when the server already has accounts")
	}
}

func TestBootstrapReadTreatsRevokedTokenAsGoneWhenTheUserIsUnreachable(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := create(t, bootstrapArgs(server.URL))

	fake.mu.Lock()
	delete(fake.tokens, state.Token)
	fake.failPassword = true
	fake.mu.Unlock()

	response, err := (Bootstrap{}).Read(context.Background(), infer.ReadRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: state.BootstrapArgs, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "" {
		t.Fatal("a revoked token without a working password must mark the resource as gone")
	}
}

func readBootstrap(t *testing.T, state BootstrapState) infer.ReadResponse[BootstrapArgs, BootstrapState] {
	t.Helper()
	response, err := (Bootstrap{}).Read(context.Background(), infer.ReadRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: state.BootstrapArgs, State: state})
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestBootstrapRepairsATokenDisabledByAServerReset(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := create(t, bootstrapArgs(server.URL))

	// A recreated container comes back with the default configuration.
	fake.mu.Lock()
	fake.tokensOn = false
	fake.mu.Unlock()

	read := readBootstrap(t, state)
	if read.ID != state.UserID || !read.State.RepairRequired || read.State.Token != state.Token {
		t.Fatalf("expected the resource to be kept and marked for repair, got %#v", read.State)
	}
	diff, err := (Bootstrap{}).Diff(context.Background(), infer.DiffRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: state.BootstrapArgs, State: read.State})
	if err != nil {
		t.Fatal(err)
	}
	if !diff.HasChanges || len(diff.DetailedDiff) != 0 {
		t.Fatalf("a repair must schedule an update without input changes, got %#v", diff)
	}
	fake.resetCalls()

	updated, err := (Bootstrap{}).Update(context.Background(), infer.UpdateRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: state.BootstrapArgs, State: read.State})
	if err != nil {
		t.Fatal(err)
	}
	if !fake.called("POST /api/v4/users/login") || !fake.tokensOn {
		t.Fatalf("expected a password login that enables personal access tokens, calls: %v", fake.calls)
	}
	if updated.Output.Token != state.Token || updated.Output.TokenID != state.TokenID || updated.Output.RepairRequired {
		t.Fatalf("a token that works again must be kept, got %#v", updated.Output)
	}
	if fake.called("POST /api/v4/users/user-infrastructure/tokens") {
		t.Fatal("no new token must be issued while the old one works")
	}
	if readBootstrap(t, updated.Output).State.RepairRequired {
		t.Fatal("the repaired resource must read back clean")
	}
}

func TestBootstrapRepairsARevokedTokenThroughThePassword(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := create(t, bootstrapArgs(server.URL))

	fake.mu.Lock()
	delete(fake.tokens, state.Token)
	fake.mu.Unlock()

	read := readBootstrap(t, state)
	if read.ID != state.UserID || !read.State.RepairRequired {
		t.Fatalf("expected the resource to be marked for repair, got %#v", read.State)
	}

	updated, err := (Bootstrap{}).Update(context.Background(), infer.UpdateRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: state.BootstrapArgs, State: read.State})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Output.Token == state.Token || updated.Output.Token == "" || updated.Output.RepairRequired {
		t.Fatalf("expected a reissued token, got %#v", updated.Output)
	}
	if fake.tokens[updated.Output.Token] != state.UserID {
		t.Fatal("the reissued token must belong to the user")
	}
}

func TestBootstrapRepairsThroughAdminTokenWhenThePasswordIsUnknown(t *testing.T) {
	fake := newFakeMattermost(t)
	adminToken := fake.addHumanAdmin()
	fake.tokensOn = true
	server := httptest.NewServer(fake)
	defer server.Close()
	args := bootstrapArgs(server.URL)
	args.AdminToken = adminToken
	state := create(t, args)

	fake.mu.Lock()
	delete(fake.tokens, state.Token)
	fake.failPassword = true
	fake.mu.Unlock()

	read := readBootstrap(t, state)
	if read.ID != state.UserID || !read.State.RepairRequired {
		t.Fatalf("expected adminToken to reach the user, got %#v", read.State)
	}
	updated, err := (Bootstrap{}).Update(context.Background(), infer.UpdateRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: args, State: read.State})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Output.Token == state.Token || fake.tokens[updated.Output.Token] != state.UserID {
		t.Fatalf("expected a reissued token of the user, got %#v", updated.Output)
	}
}

func TestBootstrapDiffReportsInputChangesLikeTheDefault(t *testing.T) {
	state := BootstrapState{BootstrapArgs: bootstrapArgs("https://chat.example.com")}
	inputs := state.BootstrapArgs
	inputs.Email = "ops@example.com"
	inputs.Username = "renamed"
	diff, err := (Bootstrap{}).Diff(context.Background(), infer.DiffRequest[BootstrapArgs, BootstrapState]{Inputs: inputs, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if !diff.HasChanges || len(diff.DetailedDiff) != 2 || diff.DetailedDiff["email"].Kind != p.Update || diff.DetailedDiff["username"].Kind != p.UpdateReplace {
		t.Fatalf("unexpected diff %#v", diff)
	}
	same, err := (Bootstrap{}).Diff(context.Background(), infer.DiffRequest[BootstrapArgs, BootstrapState]{Inputs: state.BootstrapArgs, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if same.HasChanges {
		t.Fatalf("unchanged inputs must not diff, got %#v", same)
	}
}

func TestBootstrapReadSyncsUserAndKeepsSecrets(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := create(t, bootstrapArgs(server.URL))

	fake.mu.Lock()
	fake.users["infrastructure"].email = "renamed@example.com"
	fake.users["infrastructure"].roles = "system_user"
	fake.mu.Unlock()

	response, err := (Bootstrap{}).Read(context.Background(), infer.ReadRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: state.BootstrapArgs, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != state.UserID || response.Inputs.Email != "renamed@example.com" {
		t.Fatalf("unexpected read result %#v", response.Inputs)
	}
	if !rolesEqual(response.Inputs.Roles, []SystemRole{SystemRoleUser}) {
		t.Fatalf("expected drifted roles to be read back, got %v", response.Inputs.Roles)
	}
	if response.State.Token != state.Token || response.State.GeneratedPassword != state.GeneratedPassword {
		t.Fatal("read must keep the token and the generated password")
	}
}

func TestBootstrapUpdateRotatesTokenAndAppliesChanges(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := create(t, bootstrapArgs(server.URL))
	fake.resetCalls()

	inputs := state.BootstrapArgs
	inputs.TokenDescription = "pulumi-2"
	inputs.Roles = normalizeRoles([]SystemRole{SystemRoleUser, SystemRoleAdmin, SystemRolePostAll})
	inputs.Email = "ops@example.com"
	inputs.Password = "Rotated-1!"
	response, err := (Bootstrap{}).Update(context.Background(), infer.UpdateRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: inputs, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if response.Output.Token == state.Token || response.Output.TokenID == state.TokenID {
		t.Fatal("expected a new token")
	}
	if _, stillValid := fake.tokens[state.Token]; stillValid {
		t.Fatal("expected the previous token to be revoked")
	}
	user := fake.users["infrastructure"]
	if user.roles != "system_admin system_post_all system_user" || user.email != "ops@example.com" {
		t.Fatalf("unexpected user %#v", user)
	}
	if user.password != "Rotated-1!" || response.Output.GeneratedPassword != "" {
		t.Fatalf("expected the configured password to replace the generated one, got %q", user.password)
	}
	if fake.called("POST /api/v4/users/login") {
		t.Fatal("update must use the token while it is valid")
	}
}

func TestBootstrapUpdateFallsBackToPasswordLogin(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := create(t, bootstrapArgs(server.URL))
	fake.failToken = true

	inputs := state.BootstrapArgs
	inputs.Email = "changed@example.com"
	if _, err := (Bootstrap{}).Update(context.Background(), infer.UpdateRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: inputs, State: state}); err != nil {
		t.Fatal(err)
	}
	if !fake.called("POST /api/v4/users/login") {
		t.Fatal("expected a fallback to the password login")
	}
}

func TestBootstrapUpdateFallsBackToAdminToken(t *testing.T) {
	fake := newFakeMattermost(t)
	adminToken := fake.addHumanAdmin()
	fake.tokensOn = true
	server := httptest.NewServer(fake)
	defer server.Close()
	args := bootstrapArgs(server.URL)
	args.AdminToken = adminToken
	state := create(t, args)
	fake.failToken = true
	fake.failPassword = true

	inputs := state.BootstrapArgs
	inputs.Email = "changed@example.com"
	if _, err := (Bootstrap{}).Update(context.Background(), infer.UpdateRequest[BootstrapArgs, BootstrapState]{ID: state.UserID, Inputs: inputs, State: state}); err != nil {
		t.Fatal(err)
	}
	if fake.users["infrastructure"].email != "changed@example.com" {
		t.Fatal("expected the admin token to carry out the update")
	}
}

func TestBootstrapDeleteRevokesTokenAndKeepsUser(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := create(t, bootstrapArgs(server.URL))

	if _, err := (Bootstrap{}).Delete(context.Background(), infer.DeleteRequest[BootstrapState]{ID: state.UserID, State: state}); err != nil {
		t.Fatal(err)
	}
	if _, stillValid := fake.tokens[state.Token]; stillValid {
		t.Fatal("expected the token to be revoked")
	}
	if _, kept := fake.users["infrastructure"]; !kept {
		t.Fatal("delete must keep the user")
	}
}

func TestBootstrapCheckAppliesDefaults(t *testing.T) {
	inputs := mustInputs(t, map[string]string{"username": "infrastructure", "email": "infrastructure@example.com"})
	response, err := (Bootstrap{}).Check(context.Background(), infer.CheckRequest{NewInputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Failures) != 0 {
		t.Fatalf("password and adminToken are optional, got %#v", response.Failures)
	}
	if response.Inputs.TokenDescription != "pulumi" || !rolesEqual(response.Inputs.Roles, defaultBootstrapRoles()) {
		t.Fatalf("defaults not applied: %#v", response.Inputs)
	}
}

func TestGeneratePasswordSatisfiesEveryPolicy(t *testing.T) {
	for range 20 {
		password, err := generatePassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(password) != generatedPasswordLength ||
			!strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz") ||
			!strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") ||
			!strings.ContainsAny(password, "0123456789") ||
			!strings.ContainsAny(password, "!#$%&*+-=?@^_~") {
			t.Fatalf("weak password %q", password)
		}
	}
}

func TestConfigureWithoutTokenFailsAtFirstRequest(t *testing.T) {
	config := Config{BaseURL: "https://mattermost.example.com"}
	t.Setenv(envToken, "")
	if err := config.Configure(context.Background()); err != nil {
		t.Fatalf("configure without token must succeed, got %v", err)
	}
	_, _, err := config.client.API.GetMe(context.Background(), "")
	if !errors.Is(err, mm.ErrMissingToken) {
		t.Fatalf("expected ErrMissingToken, got %v", err)
	}
}
