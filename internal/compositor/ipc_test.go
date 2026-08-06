package compositor

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestRemoveStaleIPCSocketRemovesDeadPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ipc.sock")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleIPCSocket(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("stale IPC path still exists: %v", err)
	}
}

func TestRemoveStaleIPCSocketPreservesLiveServer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ipc.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skip("sandbox does not permit Unix socket listeners")
		}
		t.Fatal(err)
	}
	defer listener.Close()

	err = removeStaleIPCSocket(path)
	if err == nil || !strings.Contains(err.Error(), "already listening") {
		t.Fatalf("live IPC server was not detected: %v", err)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("live IPC socket was removed: %v", err)
	}
}

func TestIPCResponseHelpers(t *testing.T) {
	success := ipcSuccess("request-1", map[string]bool{"ready": true})
	if success.Success == nil || !*success.Success || success.Error != "" || success.ID != "request-1" {
		t.Fatalf("invalid success response: %+v", success)
	}
	failure := ipcError(42, "failed")
	if failure.Success == nil || *failure.Success || failure.Error != "failed" || failure.ID != 42 {
		t.Fatalf("invalid error response: %+v", failure)
	}
}

func TestIPCOutputDoesNotAdvertiseSyntheticFallbackOutput(t *testing.T) {
	server := &Server{}
	server.fallbackOutput.Usable = usableBox{x: 12, y: 34, width: 900, height: 700}
	got := server.ipcOutput()
	if got != (IPCOutput{}) {
		t.Fatalf("unexpected output state: %+v", got)
	}
}

func TestIPCOutputUsesZeroSizeAsFallback(t *testing.T) {
	server := &Server{}
	got := server.ipcOutput()
	if got != (IPCOutput{}) {
		t.Fatalf("empty server output = %+v", got)
	}
}

func TestIPCHandshake(t *testing.T) {
	server := &Server{}
	response := server.handleIPCRequest(nil, IPCRequest{
		Type: "HeLLo", ID: "hello-1", ProtocolVersion: ipcProtocolVersion,
	})
	if response.Success == nil || !*response.Success || response.ProtocolVersion != ipcProtocolVersion ||
		response.Server != "HatWM" || response.ServerVersion != Version ||
		len(response.Capabilities) == 0 {
		t.Fatalf("invalid handshake response: %+v", response)
	}
}

func TestIPCRejectsIncompatibleVersionAndUnknownRequest(t *testing.T) {
	server := &Server{}
	version := server.handleIPCRequest(nil, IPCRequest{Type: "hello", ID: 1, ProtocolVersion: ipcProtocolVersion + 1})
	if version.Success == nil || *version.Success || !strings.Contains(version.Error, "unsupported protocol version") {
		t.Fatalf("invalid version response: %+v", version)
	}
	unknown := server.handleIPCRequest(nil, IPCRequest{Type: "not-a-request", ID: 2})
	if unknown.Success == nil || *unknown.Success || unknown.Error != "unknown request type" {
		t.Fatalf("invalid unknown-request response: %+v", unknown)
	}
}

func TestIPCRejectsUnsupportedCommand(t *testing.T) {
	server := &Server{}
	response := server.handleIPCCommand(IPCRequest{ID: "command-1", Command: "launch-missiles"})
	if response.Success == nil || *response.Success || response.Error != "unsupported command" {
		t.Fatalf("invalid unsupported-command response: %+v", response)
	}
}
