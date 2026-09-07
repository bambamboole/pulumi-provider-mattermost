package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestCommandCreatePostsTheCommandAndKeepsTheToken(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"POST /api/v4/commands": `{"id":"cmd-1","token":"cmd-token","creator_id":"user-1","team_id":"team-1","trigger":"answer","method":"P","url":"https://example.com/answer","auto_complete":true,"auto_complete_hint":"<text>","display_name":"Answer"}`,
	})

	response, err := (Command{}).Create(testContext(t, server.URL), infer.CreateRequest[CommandArgs]{
		Inputs: CommandArgs{
			TeamID: "team-1", Trigger: "answer", URL: "https://example.com/answer",
			AutoComplete: true, AutoCompleteHint: "<text>", DisplayName: "Answer",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "POST /api/v4/commands")
	body := server.bodies["POST /api/v4/commands"]
	if body["team_id"] != "team-1" || body["trigger"] != "answer" || body["url"] != "https://example.com/answer" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if body["method"] != "P" {
		t.Fatalf("method should default to P, got %#v", body["method"])
	}
	if body["auto_complete"] != true || body["auto_complete_hint"] != "<text>" {
		t.Fatalf("unexpected autocomplete fields: %#v", body)
	}
	if response.ID != "cmd-1" || response.Output.Token != "cmd-token" || response.Output.CreatorID != "user-1" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestCommandCreateDryRunDoesNotCallTheServer(t *testing.T) {
	server := newRecordingServer(t, map[string]string{})

	response, err := (Command{}).Create(testContext(t, server.URL), infer.CreateRequest[CommandArgs]{
		DryRun: true,
		Inputs: CommandArgs{TeamID: "team-1", Trigger: "answer", URL: "https://example.com/answer"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t)
	if response.Output.Trigger != "answer" || response.Output.Token != "" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestCommandUpdateSendsTheIDAndKeepsTheStateToken(t *testing.T) {
	server := newRecordingServer(t, map[string]string{
		"PUT /api/v4/commands/cmd-1": `{"id":"cmd-1","creator_id":"user-1","team_id":"team-1","trigger":"answer","method":"P","url":"https://example.com/v2/answer"}`,
	})

	response, err := (Command{}).Update(testContext(t, server.URL), infer.UpdateRequest[CommandArgs, CommandState]{
		ID:     "cmd-1",
		State:  CommandState{CreatorID: "user-1", Token: "cmd-token"},
		Inputs: CommandArgs{TeamID: "team-1", Trigger: "answer", URL: "https://example.com/v2/answer", Method: "P"},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.assertCalls(t, "PUT /api/v4/commands/cmd-1")
	body := server.bodies["PUT /api/v4/commands/cmd-1"]
	if body["id"] != "cmd-1" || body["url"] != "https://example.com/v2/answer" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if response.Output.Token != "cmd-token" || response.Output.URL != "https://example.com/v2/answer" {
		t.Fatalf("unexpected state: %#v", response.Output)
	}
}

func TestCommandReadSupportsImportAndToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/commands/cmd-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmd-1","token":"cmd-token","creator_id":"user-1","team_id":"team-1","trigger":"answer","method":"P","url":"https://example.com/answer","auto_complete":true,"auto_complete_desc":"Mail the customer","auto_complete_hint":"<text>","display_name":"Answer","description":"Mails a reply","username":"support","icon_url":"https://example.com/icon.png"}`))
	}))
	defer server.Close()

	response, err := (Command{}).Read(testContext(t, server.URL), infer.ReadRequest[CommandArgs, CommandState]{ID: "cmd-1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "cmd-1" || response.Inputs.TeamID != "team-1" || response.Inputs.Trigger != "answer" {
		t.Fatalf("unexpected inputs: %#v", response.Inputs)
	}
	if response.Inputs.URL != "https://example.com/answer" || response.Inputs.Method != "P" || !response.Inputs.AutoComplete {
		t.Fatalf("unexpected inputs: %#v", response.Inputs)
	}
	if response.State.Token != "cmd-token" || response.State.CreatorID != "user-1" {
		t.Fatalf("unexpected state: %#v", response.State)
	}
}

func TestCommandReadKeepsSanitizedFieldsFromState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmd-1","team_id":"team-1","trigger":"answer","display_name":"Answer"}`))
	}))
	defer server.Close()

	response, err := (Command{}).Read(testContext(t, server.URL), infer.ReadRequest[CommandArgs, CommandState]{
		ID:     "cmd-1",
		Inputs: CommandArgs{URL: "https://example.com/answer", Method: "P"},
		State:  CommandState{CreatorID: "user-1", Token: "cmd-token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Inputs.URL != "https://example.com/answer" || response.Inputs.Method != "P" {
		t.Fatalf("refresh discarded the sanitized inputs: %#v", response.Inputs)
	}
	if response.State.Token != "cmd-token" || response.State.CreatorID != "user-1" {
		t.Fatalf("refresh discarded the sanitized state: %#v", response.State)
	}
}

func TestCommandReadReportsDeletedCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"id":"api.command.get.not_found","message":"not found","status_code":404}`))
	}))
	defer server.Close()

	response, err := (Command{}).Read(testContext(t, server.URL), infer.ReadRequest[CommandArgs, CommandState]{ID: "cmd-1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "" {
		t.Fatalf("expected an empty response for a deleted command, got %#v", response)
	}
}

func TestCommandDeleteToleratesMissingCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v4/commands/cmd-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"id":"api.command.delete.not_found","message":"not found","status_code":404}`))
	}))
	defer server.Close()

	if _, err := (Command{}).Delete(testContext(t, server.URL), infer.DeleteRequest[CommandState]{ID: "cmd-1"}); err != nil {
		t.Fatal(err)
	}
}
