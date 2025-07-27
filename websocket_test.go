package wsc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin for testing
	},
}

func TestCloseConnection(t *testing.T) {
	// Set up a test WebSocket server
	server := createTestServer(t, func(c *websocket.Conn) {
		c.Close()
	})
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	chDisconnected := make(chan bool)
	chConnected := make(chan bool)

	// Create connection with receive handler
	conn := CreateConnection(ctx, wsURL, WithLogger(func(ctx context.Context, logType LogType, msg string) {
		t.Logf("[%s] %s", logType, msg)
		switch logType {
		case LogTypeDisconnected:
			chDisconnected <- true
		case LogTypeConnected:
			chConnected <- true
		}
	}))

	var err error
	go func() {
		err = conn.Run()
	}()

	select {
	case <-chConnected:
		t.Log("connected")
	case <-time.After(time.Second * 3):
		t.Error("Timeout waiting for connect")
	}

	select {
	case <-chDisconnected:
		t.Log("disconnected")
	case <-time.After(time.Second * 3):
		t.Error("Timeout waiting for disconnect")
	}

	t.Log(err)
}

func TestCancelContext(t *testing.T) {
	// Set up a test WebSocket server
	server := createTestServer(t, nil)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chConnected := make(chan bool)

	// Create connection with receive handler
	conn := CreateConnection(ctx, wsURL,
		WithLogger(func(ctx context.Context, logType LogType, msg string) {
			t.Logf("[%s] %s", logType, msg)
			if logType == LogTypeConnected {
				chConnected <- true
			}
		}),
	)

	// Run connection in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- conn.Run()
	}()

	select {
	case <-chConnected:
	case <-time.After(time.Second * 3):
		t.Error("Timeout waiting for connection")
	}

	// Cancel context to terminate connection
	cancel()

	// Wait for Run to complete
	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(time.Second * 3):
		t.Error("Connection did not terminate within timeout")
	}
}

func createTestServer(t *testing.T, fn func(c *websocket.Conn)) *httptest.Server {
	// Set up a test WebSocket server
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		time.Sleep(time.Millisecond * 200)

		t.Logf("Connection upgraded")

		// Echo server - read messages and send them back
		for {
			time.Sleep(time.Millisecond * 50)
			if fn != nil {
				fn(conn)
			}
		}
	}))
}
