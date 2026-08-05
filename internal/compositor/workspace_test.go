package compositor

import "testing"

func TestValidWorkspaceUsesConfiguredBounds(t *testing.T) {
	server := &Server{config: Config{WorkspaceCount: 4}}
	for number := -1; number <= 6; number++ {
		want := number >= 1 && number <= 4
		if got := server.validWorkspace(number); got != want {
			t.Errorf("validWorkspace(%d) = %v, want %v", number, got, want)
		}
	}
}

func TestValidWorkspaceFallsBackToNine(t *testing.T) {
	server := &Server{config: Config{WorkspaceCount: 0}}
	if !server.validWorkspace(1) || !server.validWorkspace(9) || server.validWorkspace(10) {
		t.Fatal("invalid default workspace bounds")
	}
}

func TestWorkspaceArgumentValidationDoesNotMutateState(t *testing.T) {
	server := &Server{config: Config{WorkspaceCount: 3}, currentWorkspace: 2}
	for _, arg := range []string{"", "two", "0", "4"} {
		if server.switchWorkspaceArg(arg) {
			t.Errorf("switchWorkspaceArg(%q) unexpectedly succeeded", arg)
		}
		if server.currentWorkspace != 2 {
			t.Fatalf("invalid argument %q changed workspace to %d", arg, server.currentWorkspace)
		}
	}
}
