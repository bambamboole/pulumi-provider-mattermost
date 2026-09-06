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

	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

// fakeMattermost is a minimal in-memory Mattermost API for the bootstrap flow.
type fakeMattermost struct {
	t *testing.T

	mu        sync.Mutex
	accounts  bool
	adminID   string
	bots      map[string]map[string]any // username -> bot json
	roles     map[string]string         // user id -> roles
	tokens    map[string]string         // token value -> user id
	nextToken int
	calls     []string
	failToken bool // reject the stored bot token
}

func newFakeMattermost(t *testing.T) *fakeMattermost {
	return &fakeMattermost{
		t:      t,
		bots:   map[string]map[string]any{},
		roles:  map[string]string{},
		tokens: map[string]string{},
	}
}

func (f *fakeMattermost) authorizedUser(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
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

	switch {
	case r.Method == http.MethodGet && path == "/config/client":
		noAccounts := "false"
		if !f.accounts {
			noAccounts = "true"
		}
		writeJSON(w, http.StatusOK, map[string]string{"NoAccounts": noAccounts})
		return
	case r.Method == http.MethodPost && path == "/users/login":
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !f.accounts || body["login_id"] != "admin" || body["password"] != "secret" {
			writeError(w, http.StatusUnauthorized, "api.user.login.invalid_credentials")
			return
		}
		w.Header().Set("Token", f.issueToken(f.adminID))
		writeJSON(w, http.StatusOK, map[string]any{"id": f.adminID, "username": "admin", "roles": f.roles[f.adminID]})
		return
	case r.Method == http.MethodPost && path == "/users":
		if f.accounts {
			writeError(w, http.StatusForbidden, "api.user.create_user.no_open_server")
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["username"] != "admin" || body["email"] != "admin@example.com" || body["password"] != "secret" {
			f.t.Errorf("unexpected signup body %v", body)
		}
		f.accounts = true
		f.adminID = "admin-1"
		f.roles[f.adminID] = "system_admin system_user"
		writeJSON(w, http.StatusCreated, map[string]any{"id": f.adminID, "username": "admin", "roles": f.roles[f.adminID]})
		return
	}

	userID, ok := f.authorizedUser(r)
	if !ok || (f.failToken && strings.HasPrefix(userID, "bot-")) {
		writeError(w, http.StatusUnauthorized, "api.context.session_expired.app_error")
		return
	}

	switch {
	case r.Method == http.MethodGet && path == "/users/me":
		writeJSON(w, http.StatusOK, map[string]any{"id": userID, "roles": f.roles[userID], "is_bot": strings.HasPrefix(userID, "bot-")})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/users/username/"):
		username := strings.TrimPrefix(path, "/users/username/")
		if bot, ok := f.bots[username]; ok {
			writeJSON(w, http.StatusOK, map[string]any{"id": bot["user_id"], "username": username, "is_bot": true, "roles": f.roles[bot["user_id"].(string)]})
			return
		}
		if username == "admin" && f.accounts {
			writeJSON(w, http.StatusOK, map[string]any{"id": f.adminID, "username": "admin", "roles": f.roles[f.adminID]})
			return
		}
		writeError(w, http.StatusNotFound, "app.user.get_by_username.app_error")
	case r.Method == http.MethodPost && path == "/bots":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		username := body["username"].(string)
		id := "bot-" + username
		bot := map[string]any{"user_id": id, "username": username, "display_name": body["display_name"], "description": body["description"], "owner_id": userID}
		f.bots[username] = bot
		f.roles[id] = "system_user"
		writeJSON(w, http.StatusCreated, bot)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/bots/"):
		for _, bot := range f.bots {
			if bot["user_id"] == strings.TrimPrefix(path, "/bots/") {
				writeJSON(w, http.StatusOK, bot)
				return
			}
		}
		writeError(w, http.StatusNotFound, "store.sql_bot.get.missing.app_error")
	case r.Method == http.MethodPut && strings.HasPrefix(path, "/bots/"):
		var patch map[string]any
		_ = json.NewDecoder(r.Body).Decode(&patch)
		for _, bot := range f.bots {
			if bot["user_id"] == strings.TrimPrefix(path, "/bots/") {
				for _, key := range []string{"display_name", "description"} {
					if value, ok := patch[key]; ok {
						bot[key] = value
					}
				}
				writeJSON(w, http.StatusOK, bot)
				return
			}
		}
		writeError(w, http.StatusNotFound, "store.sql_bot.get.missing.app_error")
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/users/") && !strings.Contains(strings.TrimPrefix(path, "/users/"), "/"):
		id := strings.TrimPrefix(path, "/users/")
		roles, ok := f.roles[id]
		if !ok {
			writeError(w, http.StatusNotFound, "app.user.missing_account.const")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "roles": roles, "is_bot": strings.HasPrefix(id, "bot-")})
	case r.Method == http.MethodPut && strings.HasSuffix(path, "/roles"):
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/roles")
		f.roles[id] = body["roles"]
		writeJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/tokens") && path != "/users/tokens":
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/tokens")
		token := f.issueToken(id)
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

func bootstrapArgs(baseURL string) BootstrapArgs {
	return BootstrapArgs{
		BaseURL:          baseURL,
		AdminUsername:    "admin",
		AdminEmail:       "admin@example.com",
		AdminPassword:    "secret",
		BotUsername:      "pulumi",
		BotDisplayName:   "Pulumi",
		Roles:            defaultBootstrapRoles(),
		TokenDescription: "pulumi",
	}
}

func TestBootstrapCreateSignsUpFirstAdminOnFreshServer(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()

	response, err := (Bootstrap{}).Create(context.Background(), infer.CreateRequest[BootstrapArgs]{Inputs: bootstrapArgs(server.URL)})
	if err != nil {
		t.Fatal(err)
	}
	if !fake.called("POST /api/v4/users") {
		t.Fatal("expected the first admin to be signed up")
	}
	if response.ID != "bot-pulumi" || response.Output.BotUserID != "bot-pulumi" || response.Output.AdminUserID != "admin-1" {
		t.Fatalf("unexpected state %#v", response.Output)
	}
	if response.Output.Token == "" || fake.tokens[response.Output.Token] != "bot-pulumi" {
		t.Fatalf("expected a bot token, got %#v", response.Output)
	}
	if got := fake.roles["bot-pulumi"]; got != "system_admin system_post_all system_user" {
		t.Fatalf("unexpected bot roles %q", got)
	}
}

func TestBootstrapCreateAdoptsExistingBotWithAdminToken(t *testing.T) {
	fake := newFakeMattermost(t)
	fake.accounts = true
	fake.adminID = "admin-1"
	fake.roles["admin-1"] = "system_admin system_user"
	fake.bots["pulumi"] = map[string]any{"user_id": "bot-pulumi", "username": "pulumi", "display_name": "Old", "description": "", "owner_id": "admin-1"}
	fake.roles["bot-pulumi"] = "system_admin system_post_all system_user"
	adminToken := fake.issueToken("admin-1")
	server := httptest.NewServer(fake)
	defer server.Close()

	args := bootstrapArgs(server.URL)
	args.AdminPassword = ""
	args.AdminToken = adminToken
	response, err := (Bootstrap{}).Create(context.Background(), infer.CreateRequest[BootstrapArgs]{Inputs: args})
	if err != nil {
		t.Fatal(err)
	}
	if fake.called("POST /api/v4/users") || fake.called("POST /api/v4/users/login") || fake.called("POST /api/v4/bots") {
		t.Fatalf("expected the existing bot to be adopted, calls: %v", fake.calls)
	}
	if fake.called("PUT /api/v4/users/bot-pulumi/roles") {
		t.Fatal("roles already matched and must not be rewritten")
	}
	if fake.bots["pulumi"]["display_name"] != "Pulumi" {
		t.Fatal("declared display name was not applied to the adopted bot")
	}
	if response.Output.BotDisplayName != "Pulumi" || response.Output.AdminUserID != "admin-1" || response.Output.Token == "" {
		t.Fatalf("unexpected state %#v", response.Output)
	}
}

func TestBootstrapCreateLogsInWithPasswordOnExistingServer(t *testing.T) {
	fake := newFakeMattermost(t)
	fake.accounts = true
	fake.adminID = "admin-1"
	fake.roles["admin-1"] = "system_admin system_user"
	server := httptest.NewServer(fake)
	defer server.Close()

	response, err := (Bootstrap{}).Create(context.Background(), infer.CreateRequest[BootstrapArgs]{Inputs: bootstrapArgs(server.URL)})
	if err != nil {
		t.Fatal(err)
	}
	if fake.called("POST /api/v4/users") {
		t.Fatal("must not try to sign up on a server that has accounts")
	}
	if !fake.called("POST /api/v4/bots") || response.Output.BotUserID != "bot-pulumi" {
		t.Fatalf("expected the bot to be created, state %#v", response.Output)
	}
}

func TestBootstrapCreateRejectsWrongPasswordOnExistingServer(t *testing.T) {
	fake := newFakeMattermost(t)
	fake.accounts = true
	fake.adminID = "admin-1"
	server := httptest.NewServer(fake)
	defer server.Close()

	args := bootstrapArgs(server.URL)
	args.AdminPassword = "wrong"
	_, err := (Bootstrap{}).Create(context.Background(), infer.CreateRequest[BootstrapArgs]{Inputs: args})
	if err == nil || !strings.Contains(err.Error(), "already has accounts") {
		t.Fatalf("expected a login error, got %v", err)
	}
	if fake.called("POST /api/v4/users") {
		t.Fatal("must not sign up when the server already has accounts")
	}
}

func TestBootstrapCreateRefusesHumanUsername(t *testing.T) {
	fake := newFakeMattermost(t)
	fake.accounts = true
	fake.adminID = "admin-1"
	fake.roles["admin-1"] = "system_admin system_user"
	server := httptest.NewServer(fake)
	defer server.Close()

	args := bootstrapArgs(server.URL)
	args.BotUsername = "admin"
	_, err := (Bootstrap{}).Create(context.Background(), infer.CreateRequest[BootstrapArgs]{Inputs: args})
	if err == nil || !strings.Contains(err.Error(), "is not a bot") {
		t.Fatalf("expected a not-a-bot error, got %v", err)
	}
}

func bootstrappedState(t *testing.T, fake *fakeMattermost, baseURL string) BootstrapState {
	t.Helper()
	response, err := (Bootstrap{}).Create(context.Background(), infer.CreateRequest[BootstrapArgs]{Inputs: bootstrapArgs(baseURL)})
	if err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	fake.calls = nil
	fake.mu.Unlock()
	return response.Output
}

func TestBootstrapReadTreatsRevokedTokenAsGone(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := bootstrappedState(t, fake, server.URL)

	fake.mu.Lock()
	delete(fake.tokens, state.Token)
	fake.mu.Unlock()

	response, err := (Bootstrap{}).Read(context.Background(), infer.ReadRequest[BootstrapArgs, BootstrapState]{ID: state.BotUserID, Inputs: state.BootstrapArgs, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "" {
		t.Fatal("a revoked token must mark the resource as gone")
	}
}

func TestBootstrapReadSyncsBotAndRoles(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := bootstrappedState(t, fake, server.URL)

	fake.mu.Lock()
	fake.bots["pulumi"]["display_name"] = "Renamed"
	fake.roles["bot-pulumi"] = "system_user"
	fake.mu.Unlock()

	response, err := (Bootstrap{}).Read(context.Background(), infer.ReadRequest[BootstrapArgs, BootstrapState]{ID: state.BotUserID, Inputs: state.BootstrapArgs, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != state.BotUserID || response.Inputs.BotDisplayName != "Renamed" {
		t.Fatalf("unexpected read result %#v", response.Inputs)
	}
	if !rolesEqual(response.Inputs.Roles, []SystemRole{SystemRoleUser}) {
		t.Fatalf("expected drifted roles to be read back, got %v", response.Inputs.Roles)
	}
	if response.State.Token != state.Token || response.State.AdminPassword != "secret" {
		t.Fatal("read must keep the token and admin credentials")
	}
}

func TestBootstrapUpdateRotatesTokenAndAppliesRoles(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := bootstrappedState(t, fake, server.URL)

	inputs := state.BootstrapArgs
	inputs.TokenDescription = "pulumi-2"
	inputs.Roles = normalizeRoles([]SystemRole{SystemRoleUser, SystemRoleAdmin})
	inputs.BotDescription = "Managed by Pulumi"
	response, err := (Bootstrap{}).Update(context.Background(), infer.UpdateRequest[BootstrapArgs, BootstrapState]{ID: state.BotUserID, Inputs: inputs, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if response.Output.Token == state.Token || response.Output.TokenID == state.TokenID {
		t.Fatal("expected a new token")
	}
	if _, stillValid := fake.tokens[state.Token]; stillValid {
		t.Fatal("expected the previous token to be revoked")
	}
	if fake.tokens[response.Output.Token] != "bot-pulumi" {
		t.Fatal("new token must belong to the bot")
	}
	if fake.roles["bot-pulumi"] != "system_admin system_user" {
		t.Fatalf("unexpected roles %q", fake.roles["bot-pulumi"])
	}
	if fake.bots["pulumi"]["description"] != "Managed by Pulumi" {
		t.Fatal("bot description was not patched")
	}
	if fake.called("POST /api/v4/users/login") {
		t.Fatal("update must use the bot token while it is valid")
	}
}

func TestBootstrapUpdateFallsBackToAdminLogin(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := bootstrappedState(t, fake, server.URL)
	fake.failToken = true

	inputs := state.BootstrapArgs
	inputs.BotDescription = "changed"
	if _, err := (Bootstrap{}).Update(context.Background(), infer.UpdateRequest[BootstrapArgs, BootstrapState]{ID: state.BotUserID, Inputs: inputs, State: state}); err != nil {
		t.Fatal(err)
	}
	if !fake.called("POST /api/v4/users/login") {
		t.Fatal("expected a fallback to the admin login")
	}
}

func TestBootstrapDeleteRevokesToken(t *testing.T) {
	fake := newFakeMattermost(t)
	server := httptest.NewServer(fake)
	defer server.Close()
	state := bootstrappedState(t, fake, server.URL)

	if _, err := (Bootstrap{}).Delete(context.Background(), infer.DeleteRequest[BootstrapState]{ID: state.BotUserID, State: state}); err != nil {
		t.Fatal(err)
	}
	if _, stillValid := fake.tokens[state.Token]; stillValid {
		t.Fatal("expected the token to be revoked")
	}
	if _, botKept := fake.bots["pulumi"]; !botKept {
		t.Fatal("delete must keep the bot")
	}
}

func TestBootstrapCheckRequiresCredentialsAndAppliesDefaults(t *testing.T) {
	inputs := mustInputs(t, map[string]string{"adminUsername": "admin", "adminEmail": "admin@example.com"})
	response, err := (Bootstrap{}).Check(context.Background(), infer.CheckRequest{NewInputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Failures) != 1 || response.Failures[0].Property != "adminPassword" {
		t.Fatalf("expected a credentials failure, got %#v", response.Failures)
	}
	if response.Inputs.BotUsername != "pulumi" || response.Inputs.TokenDescription != "pulumi" {
		t.Fatalf("defaults not applied: %#v", response.Inputs)
	}
	if !rolesEqual(response.Inputs.Roles, defaultBootstrapRoles()) {
		t.Fatalf("default roles not applied: %v", response.Inputs.Roles)
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
