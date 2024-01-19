package wsc

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	_ "github.com/gorilla/websocket"
	"sync"
	"time"
)

type ConnectionStatus string

type LogType string

const (
	LogTypeConnected    LogType = "connected"
	LogTypeDisconnected LogType = "disconnected"
	LogTypeInfo         LogType = "info"
)

type FnLog func(ctx context.Context, logType LogType, msg string)

const (
	ConnectionStatusDisconnected = "disconnected"
	ConnectionStatusConnected    = "connected"
)

var ErrTimeout = errors.New("timeout")

type FnReceive func(bts []byte)

type WebSocketConnection struct {
	ctx  context.Context
	url  string
	opts Options

	conn           *websocket.Conn
	status         ConnectionStatus
	chStatusChange chan ConnectionStatus

	mu sync.RWMutex
}

// CreateConnection create new connection with specified options.
func CreateConnection(ctx context.Context, url string, opts ...FnWithOption) *WebSocketConnection {
	o := Options{
		SendTimeout: defaultSendTimeout,
	}
	for _, opt := range opts {
		opt(&o)
	}

	wsc := &WebSocketConnection{
		ctx:            ctx,
		url:            url,
		opts:           o,
		chStatusChange: make(chan ConnectionStatus),
	}

	return wsc
}

// Run connection. This is a blocking call.
func (wsc *WebSocketConnection) Run() error {
	var (
		i           int
		lastConnect time.Time
	)

	for {
		lastConnect = time.Now()
		err := runConnection(wsc)

		if wsc.opts.KeepAlive {
			// If WebSocket was connected for longer than the maximum backoff, reset backoff
			durMaxBackoff := wsc.opts.Reconnect + (wsc.opts.Backoff * time.Duration(maxBackoff))
			if time.Now().Add(durMaxBackoff * -1).Before(lastConnect) {
				i = 0
			}

			sleep := wsc.opts.Reconnect + (wsc.opts.Backoff * time.Duration(i))
			wsc.log(LogTypeInfo, fmt.Sprintf("backoff %s", sleep))
			time.Sleep(sleep)
			if i < maxBackoff {
				i++
			}
		} else {
			if err != nil {
				return err
			}
			break
		}
	}

	return nil
}

// Send data over WebSocket.
func (wsc *WebSocketConnection) Send(mt int, bts []byte) error {
	err := wsc.waitForStatus(ConnectionStatusConnected, waitForStatusTimeout)
	if err != nil {
		return err
	}

	err = wsc.conn.SetWriteDeadline(time.Now().Add(wsc.opts.SendTimeout))
	if err != nil {
		return err
	}

	err = wsc.conn.WriteMessage(mt, bts)
	if err != nil {
		return err
	}

	return nil
}

func (wsc *WebSocketConnection) waitForStatus(status ConnectionStatus, timeout time.Duration) error {
	if wsc.getStatus() == status {
		return nil
	}

	for {
		select {
		case st := <-wsc.chStatusChange:
			if st == status {
				return nil
			}
		case <-time.After(timeout):
			return ErrTimeout
		}
	}
}

func (wsc *WebSocketConnection) getStatus() ConnectionStatus {
	wsc.mu.RLock()
	defer wsc.mu.RUnlock()

	return wsc.status
}

func (wsc *WebSocketConnection) setStatus(status ConnectionStatus) {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	wsc.status = status

	if status == ConnectionStatusConnected {
		wsc.log(LogTypeConnected, "connected")
	} else if status == ConnectionStatusDisconnected {
		wsc.log(LogTypeDisconnected, "disconnected")
	}

	select {
	case wsc.chStatusChange <- wsc.status:
	default:
	}
}

func (wsc *WebSocketConnection) log(logType LogType, msg string) {
	if wsc.opts.fnLog != nil {
		wsc.opts.fnLog(wsc.ctx, logType, msg)
	}
}

func runConnection(wsc *WebSocketConnection) error {
	conn, _, err := websocket.DefaultDialer.Dial(wsc.url, nil)
	if err != nil {
		return err
	}

	wsc.conn = conn

	wsc.setStatus(ConnectionStatusConnected)

	defer func() {
		_ = conn.Close()
		wsc.setStatus(ConnectionStatusDisconnected)
	}()

	readCh := make(chan []byte)
	var (
		readErr error
		stop    bool
	)
	go func() {
		readErr = readMessages(conn, readCh)
	}()

	for {
		select {
		case bts, ok := <-readCh:
			if !ok {
				stop = true
				break
			}

			if wsc.opts.FnReceive != nil {
				wsc.opts.FnReceive(bts)
			}
		}

		if stop {
			break
		}
	}

	if readErr != nil {
		return readErr
	}

	return nil
}

func readMessages(conn *websocket.Conn, rc chan []byte) error {
	defer func() {
		close(rc)
	}()

	for {
		mt, bts, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure, websocket.CloseGoingAway) {
				return err
			}

			return nil
		}

		if bts != nil {
			rc <- bts
		}

		if mt == websocket.CloseMessage {
			return nil
		}
	}
}
