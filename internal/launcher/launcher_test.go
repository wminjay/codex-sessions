package launcher

import (
	"slices"
	"testing"

	"github.com/wminjay/codex-sessions/internal/session"
)

func TestBuildCustomYoloCommand(t *testing.T) {
	item := session.Session{
		ID: "session-1", Provider: "openai", CodexHome: "/codex", SQLiteHome: "/state",
	}
	command, err := Build(Request{
		Session: item, Action: ActionResume, Provider: ProviderCustom,
	}, Options{CodexBin: "codex", CustomProfile: "my-api", Yolo: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"resume", "session-1", "-p", "my-api", "-c", `model_provider="custom"`, "--yolo",
	}
	if !slices.Equal(command.Args, want) {
		t.Fatalf("got %#v, want %#v", command.Args, want)
	}
	if slices.Contains(command.Args, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatal("the expanded dangerous flag must not replace --yolo")
	}
	assertEnvironment(t, command.Env, "CODEX_HOME", "/codex")
	assertEnvironment(t, command.Env, "CODEX_SQLITE_HOME", "/state")
}

func TestBuildUsesOriginalProvider(t *testing.T) {
	command, err := Build(Request{
		Session: session.Session{ID: "session-1", Provider: "openai"},
		Action:  ActionFork, Provider: ProviderOriginal,
	}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(command.Args, `model_provider="openai"`) {
		t.Fatalf("original provider missing from %#v", command.Args)
	}
}

func assertEnvironment(t *testing.T, environment []string, name, value string) {
	t.Helper()
	want := name + "=" + value
	if !slices.Contains(environment, want) {
		t.Fatalf("environment does not contain %q", want)
	}
}
