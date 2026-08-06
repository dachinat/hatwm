package compositor

import (
	"net"
	"sync"
	"testing"
)

func TestIPCClientEnqueueAndCloseAreSynchronized(t *testing.T) {
	for i := 0; i < 500; i++ {
		serverConn, peerConn := net.Pipe()
		client := &ipcClient{
			conn:          serverConn,
			send:          make(chan IPCMessage, 1),
			subscriptions: make(map[string]bool),
		}
		ipc := &IPCServer{clients: map[*ipcClient]struct{}{client: {}}}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			client.enqueue(IPCMessage{Type: "event"})
		}()
		go func() {
			defer wg.Done()
			ipc.removeClient(client)
		}()
		wg.Wait()
		_ = peerConn.Close()

		client.mu.RLock()
		closed := client.closed
		client.mu.RUnlock()
		if !closed {
			t.Fatal("client was not closed")
		}
	}
}

func TestIPCClientEnqueueAfterCloseIsIgnored(t *testing.T) {
	serverConn, peerConn := net.Pipe()
	defer peerConn.Close()
	client := &ipcClient{
		conn:          serverConn,
		send:          make(chan IPCMessage, 1),
		subscriptions: make(map[string]bool),
	}
	ipc := &IPCServer{clients: map[*ipcClient]struct{}{client: {}}}
	ipc.removeClient(client)
	client.enqueue(IPCMessage{Type: "late-event"})
}
