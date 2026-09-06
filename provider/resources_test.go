package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"

	mm "github.com/bambamboole/pulumi-provider-mattermost/internal/mattermost"
)

func TestMembershipParts(t *testing.T) {
	tests := []struct {
		id       string
		parentID string
		userID   string
		ok       bool
	}{
		{"team:user", "team", "user", true},
		{"channel:user", "channel", "user", true},
		{"missing", "", "", false},
		{":user", "", "", false},
		{"team:", "", "", false},
	}

	for _, test := range tests {
		parentID, userID, err := membershipParts(test.id)
		if test.ok {
			if err != nil {
				t.Fatalf("membershipParts(%q): %v", test.id, err)
			}
			if parentID != test.parentID || userID != test.userID {
				t.Fatalf("membershipParts(%q) = %q, %q", test.id, parentID, userID)
			}
		} else if err == nil {
			t.Fatalf("membershipParts(%q) unexpectedly succeeded", test.id)
		}
	}
}

func TestTeamMemberReadSupportsCompositeImportID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/teams/team-1/members/user-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"team_id":"team-1","user_id":"user-1","roles":"team_user"}`))
	}))
	defer server.Close()

	ctx := testContext(t, server.URL)
	response, err := (TeamMember{}).Read(ctx, infer.ReadRequest[TeamMemberArgs, TeamMemberState]{ID: "team-1:user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.TeamID != "team-1" || response.Inputs.UserID != "user-1" {
		t.Fatalf("unexpected inputs: %#v", response.Inputs)
	}
}

func TestChannelMemberReadSupportsCompositeImportID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/channels/channel-1/members/user-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"channel_id":"channel-1","user_id":"user-1","roles":"channel_user"}`))
	}))
	defer server.Close()

	ctx := testContext(t, server.URL)
	response, err := (ChannelMember{}).Read(ctx, infer.ReadRequest[ChannelMemberArgs, ChannelMemberState]{ID: "channel-1:user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.ChannelID != "channel-1" || response.Inputs.UserID != "user-1" {
		t.Fatalf("unexpected inputs: %#v", response.Inputs)
	}
}

func TestUserReadPreservesPasswordInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/users/user-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"user-1","username":"manuel","email":"manuel@example.com","first_name":"Manuel","last_name":"Example","nickname":"Manu"}`))
	}))
	defer server.Close()

	ctx := testContext(t, server.URL)
	response, err := (User{}).Read(ctx, infer.ReadRequest[UserArgs, UserState]{
		ID: "user-1",
		Inputs: UserArgs{Password: "super-secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.Password != "super-secret" {
		t.Fatal("refresh discarded the secret password input")
	}
	if response.Inputs.Username != "manuel" || response.Inputs.Email != "manuel@example.com" {
		t.Fatalf("unexpected user inputs: %#v", response.Inputs)
	}
}

func testContext(t *testing.T, baseURL string) context.Context {
	t.Helper()
	c, err := mm.New(baseURL, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	return context.WithValue(context.Background(), clientKey{}, c)
}
