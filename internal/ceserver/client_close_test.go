package ceserver

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestClientCloseDropsDebugConnectionWithoutCloseCommand(t *testing.T) {
	serverConnection, clientConnection := net.Pipe()
	client := &Client{connection: clientConnection, timeout: time.Second, debugActive: true}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 1)
	if _, err := serverConnection.Read(buffer); err != io.EOF {
		t.Fatalf("read error = %v, want EOF", err)
	}
	_ = serverConnection.Close()
}

func TestClientCloseSendsCloseCommandOutsideDebugSession(t *testing.T) {
	serverConnection, clientConnection := net.Pipe()
	client := &Client{connection: clientConnection, timeout: time.Second}
	closed := make(chan error, 1)
	go func() {
		closed <- client.Close()
	}()
	buffer := make([]byte, 1)
	if _, err := io.ReadFull(serverConnection, buffer); err != nil {
		t.Fatal(err)
	}
	if buffer[0] != commandCloseConnection {
		t.Fatalf("command = %d", buffer[0])
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	_ = serverConnection.Close()
}
