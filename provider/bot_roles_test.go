package provider

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

const botJSON = `{"user_id":"bot-user-1","username":"pulumi","display_name":"Pulumi","description":"","owner_id":"owner-1"}`

func TestBotCreateAppliesRoles(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/bots":                  botJSON,
		"PUT /api/v4/users/bot-user-1/roles": `{"status":"OK"}`,
	})

	response, err := (Bot{}).Create(testContext(t, server.URL), infer.CreateRequest[BotArgs]{
		Inputs: BotArgs{Username: "pulumi", Roles: []SystemRole{SystemRoleUser, SystemRoleAdmin, SystemRolePostAll}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /api/v4/bots", "PUT /api/v4/users/bot-user-1/roles")
	if server.bodies["PUT /api/v4/users/bot-user-1/roles"]["roles"] != "system_admin system_post_all system_user" {
		t.Fatalf("unexpected roles body: %#v", server.bodies["PUT /api/v4/users/bot-user-1/roles"])
	}
	if response.ID != "bot-user-1" || len(response.Output.Roles) != 3 {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestBotCreateSkipsRolesWhenUnmanagedOrDefault(t *testing.T) {
	for name, roles := range map[string][]SystemRole{
		"unset":   nil,
		"default": {SystemRoleUser},
	} {
		t.Run(name, func(t *testing.T) {
			server := newRecordingServer(t, map[string]string{"POST /api/v4/bots": botJSON})

			_, err := (Bot{}).Create(testContext(t, server.URL), infer.CreateRequest[BotArgs]{
				Inputs: BotArgs{Username: "pulumi", Roles: roles},
			})
			if err != nil {
				t.Fatal(err)
			}
			server.assertCalls(t, "POST /api/v4/bots")
		})
	}
}

func TestBotUpdateAppliesChangedRoles(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"PUT /api/v4/bots/bot-user-1":        botJSON,
		"PUT /api/v4/users/bot-user-1/roles": `{"status":"OK"}`,
	})

	_, err := (Bot{}).Update(testContext(t, server.URL), infer.UpdateRequest[BotArgs, BotState]{
		ID:     "bot-user-1",
		State:  BotState{BotArgs: BotArgs{Username: "pulumi", Roles: []SystemRole{SystemRoleUser}}, OwnerID: "owner-1"},
		Inputs: BotArgs{Username: "pulumi", Roles: []SystemRole{SystemRoleUser, SystemRoleAdmin}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "PUT /api/v4/bots/bot-user-1", "PUT /api/v4/users/bot-user-1/roles")
	if server.bodies["PUT /api/v4/users/bot-user-1/roles"]["roles"] != "system_admin system_user" {
		t.Fatalf("unexpected roles body: %#v", server.bodies["PUT /api/v4/users/bot-user-1/roles"])
	}
}

func TestBotUpdateLeavesRolesAloneWhenUnchangedOrUnmanaged(t *testing.T) {
	for name, roles := range map[string][]SystemRole{
		"unchanged": {SystemRoleUser, SystemRoleAdmin},
		"unset":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			server := newRecordingServer(t, map[string]string{"PUT /api/v4/bots/bot-user-1": botJSON})

			_, err := (Bot{}).Update(testContext(t, server.URL), infer.UpdateRequest[BotArgs, BotState]{
				ID:     "bot-user-1",
				State:  BotState{BotArgs: BotArgs{Username: "pulumi", Roles: []SystemRole{SystemRoleAdmin, SystemRoleUser}}},
				Inputs: BotArgs{Username: "pulumi", DisplayName: "Pulumi", Roles: roles},
			})
			if err != nil {
				t.Fatal(err)
			}
			server.assertCalls(t, "PUT /api/v4/bots/bot-user-1")
		})
	}
}

func TestBotReadReadsRolesOnlyWhenManaged(t *testing.T) {
	managed := newRecordingServer(t, map[string]string{
		"GET /api/v4/bots/bot-user-1":  botJSON,
		"GET /api/v4/users/bot-user-1": `{"id":"bot-user-1","username":"pulumi","roles":"system_user system_admin"}`,
	})

	response, err := (Bot{}).Read(testContext(t, managed.URL), infer.ReadRequest[BotArgs, BotState]{
		ID:     "bot-user-1",
		Inputs: BotArgs{Username: "pulumi", Roles: []SystemRole{SystemRoleUser}},
	})
	if err != nil {
		t.Fatal(err)
	}
	managed.assertCalls(t, "GET /api/v4/bots/bot-user-1", "GET /api/v4/users/bot-user-1")
	if joinRoles(response.Inputs.Roles) != "system_admin system_user" {
		t.Fatalf("unexpected roles: %#v", response.Inputs.Roles)
	}

	unmanaged := newRecordingServer(t, map[string]string{"GET /api/v4/bots/bot-user-1": botJSON})

	response, err = (Bot{}).Read(testContext(t, unmanaged.URL), infer.ReadRequest[BotArgs, BotState]{ID: "bot-user-1"})
	if err != nil {
		t.Fatal(err)
	}
	unmanaged.assertCalls(t, "GET /api/v4/bots/bot-user-1")
	if response.Inputs.Roles != nil {
		t.Fatalf("import must not take over roles: %#v", response.Inputs.Roles)
	}
}

func TestBotCheckNormalizesRolesAndKeepsUnsetUnmanaged(t *testing.T) {
	ctx := context.Background()

	withRoles, err := (Bot{}).Check(ctx, infer.CheckRequest{NewInputs: property.NewMap(map[string]property.Value{
		"username": property.New("pulumi"),
		"roles": property.New([]property.Value{
			property.New("system_post_all"), property.New("system_admin"), property.New("system_admin"),
		}),
	})})
	if err != nil {
		t.Fatal(err)
	}
	if joinRoles(withRoles.Inputs.Roles) != "system_admin system_post_all" {
		t.Fatalf("roles not normalized: %#v", withRoles.Inputs.Roles)
	}

	unset, err := (Bot{}).Check(ctx, infer.CheckRequest{NewInputs: mustInputs(t, map[string]string{"username": "pulumi"})})
	if err != nil {
		t.Fatal(err)
	}
	if unset.Inputs.Roles != nil {
		t.Fatalf("unset roles must stay unmanaged, got %#v", unset.Inputs.Roles)
	}
}
