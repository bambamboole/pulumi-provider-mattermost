package provider

import (
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestNormalizeRoles(t *testing.T) {
	got := joinRoles(normalizeRoles([]SystemRole{SystemRoleAdmin, SystemRoleUser, SystemRoleAdmin, ""}))
	if got != "system_admin system_user" {
		t.Fatalf("unexpected roles %q", got)
	}
	if got := joinRoles(normalizeRoles(nil)); got != "system_user" {
		t.Fatalf("empty roles must default to system_user, got %q", got)
	}
	if !rolesEqual(parseRoles("system_user system_admin"), []SystemRole{SystemRoleAdmin, SystemRoleUser}) {
		t.Fatal("role comparison must ignore order")
	}
}

func TestUserCreateAssignsRolesWhenNotDefault(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/users":             `{"id":"user-1","username":"alice","roles":"system_user"}`,
		"PUT /api/v4/users/user-1/roles": `{"status":"OK"}`,
	})
	_, err := (User{}).Create(testContext(t, server.URL), infer.CreateRequest[UserArgs]{
		Inputs: UserArgs{Username: "alice", Email: "alice@example.com", Roles: []SystemRole{SystemRoleAdmin, SystemRoleUser}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /api/v4/users", "PUT /api/v4/users/user-1/roles")
	if server.bodies["PUT /api/v4/users/user-1/roles"]["roles"] != "system_admin system_user" {
		t.Fatalf("unexpected roles body: %#v", server.bodies["PUT /api/v4/users/user-1/roles"])
	}
}

func TestUserCreateSkipsRolesCallForDefault(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/users": `{"id":"user-1","username":"alice","roles":"system_user"}`,
	})
	_, err := (User{}).Create(testContext(t, server.URL), infer.CreateRequest[UserArgs]{
		Inputs: UserArgs{Username: "alice", Email: "alice@example.com", Roles: []SystemRole{SystemRoleUser}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /api/v4/users")
}

func TestUserUpdateChangesRoles(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"PUT /api/v4/users/user-1/patch": `{"id":"user-1"}`,
		"PUT /api/v4/users/user-1/roles": `{"status":"OK"}`,
	})
	_, err := (User{}).Update(testContext(t, server.URL), infer.UpdateRequest[UserArgs, UserState]{
		ID:     "user-1",
		State:  UserState{UserArgs: UserArgs{Username: "alice", Email: "alice@example.com", Roles: []SystemRole{SystemRoleUser}}},
		Inputs: UserArgs{Username: "alice", Email: "alice@example.com", Roles: []SystemRole{SystemRoleUser, SystemRoleAdmin}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "PUT /api/v4/users/user-1/patch", "PUT /api/v4/users/user-1/roles")
}

func TestUserReadReturnsRoles(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"GET /api/v4/users/user-1": `{"id":"user-1","username":"alice","email":"alice@example.com","roles":"system_user system_admin"}`,
	})
	response, err := (User{}).Read(testContext(t, server.URL), infer.ReadRequest[UserArgs, UserState]{ID: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if joinRoles(response.Inputs.Roles) != "system_admin system_user" {
		t.Fatalf("unexpected roles %#v", response.Inputs.Roles)
	}
}

func TestTeamMemberSchemeAdmin(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/teams/team-1/members":                   `{"team_id":"team-1","user_id":"user-1"}`,
		"PUT /api/v4/teams/team-1/members/user-1/schemeRoles": `{"status":"OK"}`,
		"GET /api/v4/teams/team-1/members/user-1":             `{"team_id":"team-1","user_id":"user-1","scheme_user":true,"scheme_admin":true}`,
	})
	ctx := testContext(t, server.URL)
	_, err := (TeamMember{}).Create(ctx, infer.CreateRequest[TeamMemberArgs]{Inputs: TeamMemberArgs{TeamID: "team-1", UserID: "user-1", SchemeAdmin: true}})
	if err != nil {
		t.Fatal(err)
	}
	body := server.bodies["PUT /api/v4/teams/team-1/members/user-1/schemeRoles"]
	if body["scheme_admin"] != true || body["scheme_user"] != true {
		t.Fatalf("unexpected scheme roles body: %#v", body)
	}

	_, err = (TeamMember{}).Update(ctx, infer.UpdateRequest[TeamMemberArgs, TeamMemberState]{
		ID:     "team-1:user-1",
		State:  TeamMemberState{TeamMemberArgs: TeamMemberArgs{TeamID: "team-1", UserID: "user-1", SchemeAdmin: true}},
		Inputs: TeamMemberArgs{TeamID: "team-1", UserID: "user-1", SchemeAdmin: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	if server.bodies["PUT /api/v4/teams/team-1/members/user-1/schemeRoles"]["scheme_admin"] != false {
		t.Fatal("update must revoke the admin role")
	}

	read, err := (TeamMember{}).Read(ctx, infer.ReadRequest[TeamMemberArgs, TeamMemberState]{ID: "team-1:user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !read.Inputs.SchemeAdmin {
		t.Fatalf("read must report scheme admin: %#v", read.Inputs)
	}
	server.assertCalls(t,
		"POST /api/v4/teams/team-1/members",
		"PUT /api/v4/teams/team-1/members/user-1/schemeRoles",
		"PUT /api/v4/teams/team-1/members/user-1/schemeRoles",
		"GET /api/v4/teams/team-1/members/user-1",
	)
}

func TestChannelMemberSchemeAdmin(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/channels/ch-1/members":                   `{"channel_id":"ch-1","user_id":"user-1"}`,
		"PUT /api/v4/channels/ch-1/members/user-1/schemeRoles": `{"status":"OK"}`,
		"GET /api/v4/channels/ch-1/members/user-1":             `{"channel_id":"ch-1","user_id":"user-1","scheme_user":true,"scheme_admin":true}`,
	})
	ctx := testContext(t, server.URL)
	_, err := (ChannelMember{}).Create(ctx, infer.CreateRequest[ChannelMemberArgs]{Inputs: ChannelMemberArgs{ChannelID: "ch-1", UserID: "user-1", SchemeAdmin: true}})
	if err != nil {
		t.Fatal(err)
	}
	read, err := (ChannelMember{}).Read(ctx, infer.ReadRequest[ChannelMemberArgs, ChannelMemberState]{ID: "ch-1:user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !read.Inputs.SchemeAdmin {
		t.Fatalf("read must report scheme admin: %#v", read.Inputs)
	}
	server.assertCalls(t,
		"POST /api/v4/channels/ch-1/members",
		"PUT /api/v4/channels/ch-1/members/user-1/schemeRoles",
		"GET /api/v4/channels/ch-1/members/user-1",
	)
}
