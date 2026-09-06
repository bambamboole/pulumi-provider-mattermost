package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// recordingServer captures every request as "METHOD path" and decodes JSON bodies.
type recordingServer struct {
	*httptest.Server
	calls  []string
	bodies map[string]map[string]any
}

func newRecordingServer(t *testing.T, responses map[string]string) *recordingServer {
	t.Helper()
	rs := &recordingServer{bodies: map[string]map[string]any{}}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		rs.calls = append(rs.calls, key)
		if r.Body != nil && r.ContentLength != 0 {
			body := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("%s: decode body: %v", key, err)
			}
			rs.bodies[key] = body
		}
		response, ok := responses[key]
		if !ok {
			t.Fatalf("unexpected request %s", key)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(rs.Close)
	return rs
}

func (rs *recordingServer) assertCalls(t *testing.T, want ...string) {
	t.Helper()
	if strings.Join(rs.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected calls:\n%s\nwant:\n%s", strings.Join(rs.calls, "\n"), strings.Join(want, "\n"))
	}
}

func TestTeamUpdatePatchesAndChangesPrivacy(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"PUT /api/v4/teams/team-1/patch":   `{"id":"team-1"}`,
		"PUT /api/v4/teams/team-1/privacy": `{"id":"team-1"}`,
	})

	_, err := (Team{}).Update(testContext(t, server.URL), infer.UpdateRequest[TeamArgs, TeamState]{
		ID:     "team-1",
		State:  TeamState{TeamArgs: TeamArgs{Name: "eng", DisplayName: "Eng", Type: "O"}},
		Inputs: TeamArgs{Name: "eng", DisplayName: "Engineering", Description: "All engineers", Type: "I"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "PUT /api/v4/teams/team-1/patch", "PUT /api/v4/teams/team-1/privacy")
	patch := server.bodies["PUT /api/v4/teams/team-1/patch"]
	if patch["display_name"] != "Engineering" || patch["description"] != "All engineers" {
		t.Fatalf("unexpected patch body: %#v", patch)
	}
	if _, ok := patch["allow_open_invite"]; ok && patch["allow_open_invite"] != nil {
		t.Fatalf("patch must not touch unmanaged fields: %#v", patch)
	}
	if server.bodies["PUT /api/v4/teams/team-1/privacy"]["privacy"] != "I" {
		t.Fatalf("unexpected privacy body: %#v", server.bodies["PUT /api/v4/teams/team-1/privacy"])
	}
}

func TestTeamUpdateRenameKeepsUnmanagedSettings(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/teams/team-1": `{"id":"team-1","name":"eng","display_name":"Eng","type":"O","allow_open_invite":true,"allowed_domains":"example.com"}`,
		"PUT /api/v4/teams/team-1": `{"id":"team-1"}`,
	})

	_, err := (Team{}).Update(testContext(t, server.URL), infer.UpdateRequest[TeamArgs, TeamState]{
		ID:     "team-1",
		State:  TeamState{TeamArgs: TeamArgs{Name: "eng", DisplayName: "Eng", Type: "O"}},
		Inputs: TeamArgs{Name: "platform", DisplayName: "Platform", Type: "O"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "GET /api/v4/teams/team-1", "PUT /api/v4/teams/team-1")
	body := server.bodies["PUT /api/v4/teams/team-1"]
	if body["name"] != "platform" || body["display_name"] != "Platform" {
		t.Fatalf("unexpected update body: %#v", body)
	}
	if body["allow_open_invite"] != true || body["allowed_domains"] != "example.com" {
		t.Fatalf("rename must carry existing unmanaged settings: %#v", body)
	}
}

func TestChannelUpdatePatchesAndChangesPrivacy(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"PUT /api/v4/channels/ch-1/patch":   `{"id":"ch-1"}`,
		"PUT /api/v4/channels/ch-1/privacy": `{"id":"ch-1"}`,
	})

	_, err := (Channel{}).Update(testContext(t, server.URL), infer.UpdateRequest[ChannelArgs, ChannelState]{
		ID:     "ch-1",
		State:  ChannelState{ChannelArgs: ChannelArgs{TeamID: "team-1", Name: "ops", DisplayName: "Ops", Type: "O"}},
		Inputs: ChannelArgs{TeamID: "team-1", Name: "ops", DisplayName: "Operations", Header: "On call", Type: "P"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "PUT /api/v4/channels/ch-1/patch", "PUT /api/v4/channels/ch-1/privacy")
	patch := server.bodies["PUT /api/v4/channels/ch-1/patch"]
	if patch["display_name"] != "Operations" || patch["header"] != "On call" {
		t.Fatalf("unexpected patch body: %#v", patch)
	}
	if server.bodies["PUT /api/v4/channels/ch-1/privacy"]["privacy"] != "P" {
		t.Fatalf("unexpected privacy body: %#v", server.bodies["PUT /api/v4/channels/ch-1/privacy"])
	}
}

func TestChannelUpdateWithoutTypeChangeOnlyPatches(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"PUT /api/v4/channels/ch-1/patch": `{"id":"ch-1"}`,
	})

	_, err := (Channel{}).Update(testContext(t, server.URL), infer.UpdateRequest[ChannelArgs, ChannelState]{
		ID:     "ch-1",
		State:  ChannelState{ChannelArgs: ChannelArgs{TeamID: "team-1", Name: "ops", DisplayName: "Ops", Type: "O"}},
		Inputs: ChannelArgs{TeamID: "team-1", Name: "ops", DisplayName: "Ops", Purpose: "Incidents", Type: "O"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "PUT /api/v4/channels/ch-1/patch")
}

func TestUserUpdateUsesPatchWithoutPassword(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"PUT /api/v4/users/user-1/patch": `{"id":"user-1"}`,
	})

	_, err := (User{}).Update(testContext(t, server.URL), infer.UpdateRequest[UserArgs, UserState]{
		ID:     "user-1",
		State:  UserState{UserArgs: UserArgs{Username: "alice", Email: "alice@example.com", Password: "secret"}},
		Inputs: UserArgs{Username: "alice", Email: "alice@example.com", Password: "secret", FirstName: "Alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "PUT /api/v4/users/user-1/patch")
	body := server.bodies["PUT /api/v4/users/user-1/patch"]
	if body["first_name"] != "Alice" || body["username"] != "alice" {
		t.Fatalf("unexpected patch body: %#v", body)
	}
	if _, ok := body["password"]; ok {
		t.Fatalf("password must not be sent on update: %#v", body)
	}
	if _, ok := body["notify_props"]; ok {
		t.Fatalf("unmanaged fields must not be sent: %#v", body)
	}
}

func TestTeamCheckDefaultsAndValidatesType(t *testing.T) {
	ctx := testContext(t, "http://unused")
	ok, err := (Team{}).Check(ctx, infer.CheckRequest{NewInputs: mustInputs(t, map[string]string{"name": "eng", "displayName": "Eng"})})
	if err != nil {
		t.Fatal(err)
	}
	if ok.Inputs.Type != "O" || len(ok.Failures) != 0 {
		t.Fatalf("unexpected check result: %#v", ok)
	}
	bad, err := (Team{}).Check(ctx, infer.CheckRequest{NewInputs: mustInputs(t, map[string]string{"name": "eng", "displayName": "Eng", "type": "X"})})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad.Failures) != 1 || bad.Failures[0].Property != "type" {
		t.Fatalf("expected a type failure, got %#v", bad.Failures)
	}
}

func mustInputs(t *testing.T, values map[string]string) property.Map {
	t.Helper()
	m := map[string]property.Value{}
	for key, value := range values {
		m[key] = property.New(value)
	}
	return property.NewMap(m)
}

func TestUserReadTreatsDeactivatedAsMissing(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/users/user-1": `{"id":"user-1","username":"alice","email":"alice@example.com","delete_at":1700000000000}`,
	})
	response, err := (User{}).Read(testContext(t, server.URL), infer.ReadRequest[UserArgs, UserState]{ID: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "" {
		t.Fatalf("deactivated user must read as missing, got %#v", response)
	}
}

func TestChannelReadTreatsArchivedAsMissing(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/channels/ch-1": `{"id":"ch-1","team_id":"team-1","name":"ops","display_name":"Ops","type":"O","delete_at":1700000000000}`,
	})
	response, err := (Channel{}).Read(testContext(t, server.URL), infer.ReadRequest[ChannelArgs, ChannelState]{ID: "ch-1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "" {
		t.Fatalf("archived channel must read as missing, got %#v", response)
	}
}
