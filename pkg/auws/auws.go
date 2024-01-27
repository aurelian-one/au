package auws

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/gorilla/websocket"
)

// DefaultConnectionReadLimit is the maximum read size of a message on the connection. I don't actually know what the max
// message size is coming out of auto-merge. /shrug
var DefaultConnectionReadLimit int64 = 65536

// DefaultWriteTimeout is the timeout after writing a message for it to be acked.
var DefaultWriteTimeout = time.Second * 10

// DefaultPingTimeout is the timeout to wait for a next message. When quiet this is the maximum interval between pings
// from the remote side.
var DefaultPingTimeout = time.Second * 30

// DefaultPongTimeout is the timeout to wait for a pong reply for the pings we send
var DefaultPongTimeout = time.Second * 10

func writePump(ctx context.Context, logger *slog.Logger, conn *websocket.Conn, messages chan []byte, writeWait time.Duration, pingPeriod time.Duration) error {
	logger.DebugContext(ctx, "sending pings on an interval", "interval", pingPeriod)
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		logger.DebugContext(ctx, "ensure connection closed")
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.DebugContext(ctx, "error while closing connection", "err", err)
		}
	}()
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				logger.DebugContext(ctx, "sending close message")
				return conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(writeWait))
			}
			logger.DebugContext(ctx, "sending binary message", "size_bytes", len(message))
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				return err
			}
		case <-ticker.C:
			logger.DebugContext(ctx, "sending ping")
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return err
			}
		}
	}
}

func readPump(ctx context.Context, logger *slog.Logger, conn *websocket.Conn, messages chan []byte, pingWait time.Duration) error {
	defer func() {
		logger.DebugContext(ctx, "ensure connection closed")
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.DebugContext(ctx, "error while closing connection", "err", err)
		}
	}()
	conn.SetReadLimit(DefaultConnectionReadLimit)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(pingWait)); err != nil {
			logger.Debug("error while setting read deadline", "err", err)
		}
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.DebugContext(ctx, "unexpected error while reading", "err", err)
				close(messages)
				return err
			}
			logger.DebugContext(ctx, "received close")
			close(messages)
			return nil
		}
		switch messageType {
		case websocket.BinaryMessage:
			logger.DebugContext(ctx, "received binary message", "size_bytes", len(message))
			messages <- message
		default:
			logger.DebugContext(ctx, "ignoring message", "type", messageType)
		}
	}
}

func headsEqual(a []automerge.ChangeHash, b []automerge.ChangeHash) bool {
	if len(a) != len(b) {
		return false
	}
	for _, hash := range a {
		if !slices.Contains(b, hash) {
			return false
		}
	}
	return true
}

func Sync(ctx context.Context, logger *slog.Logger, conn *websocket.Conn, doc *automerge.Doc, untilCaughtUp bool) error {
	wg := new(sync.WaitGroup)

	incomingMessages := make(chan []byte)
	outGoingMessages := make(chan []byte)

	// set timeouts for this sync session
	writeWait := DefaultWriteTimeout
	pingWait := DefaultPingTimeout
	pongWait := DefaultPongTimeout
	pingPeriod := time.Duration(float32(DefaultWriteTimeout) + float32(DefaultPingTimeout-DefaultWriteTimeout)*rand.Float32())

	conn.SetPingHandler(func(appData string) error {
		logger.DebugContext(ctx, "received ping - sending pong")
		err := conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(writeWait))
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return err
	})
	conn.SetPongHandler(func(appData string) error {
		logger.DebugContext(ctx, "received pong")
		_ = conn.SetReadDeadline(time.Now().Add(pingWait))
		return nil
	})

	wg.Add(2)
	go func() {
		defer wg.Done()
		logger := logger.WithGroup("read-pump")
		logger.DebugContext(ctx, "read-pump finished", "err", readPump(ctx, logger, conn, incomingMessages, pingWait))
	}()
	go func() {
		defer wg.Done()
		logger := logger.WithGroup("write-pump")
		logger.DebugContext(ctx, "write-pump finished", "err", writePump(ctx, logger, conn, outGoingMessages, writeWait, pingPeriod))
	}()

	ss := automerge.NewSyncState(doc)
	flush := func() {
		for {
			if msg, ok := ss.GenerateMessage(); ok {
				outGoingMessages <- msg.Bytes()
			} else {
				break
			}
		}
	}

	// first flush everything we can
	flush()

	var lastError error
	for msg := range incomingMessages {
		if sm, err := ss.ReceiveMessage(msg); err != nil {
			lastError = err
			break
		} else if untilCaughtUp && headsEqual(sm.Heads(), doc.Heads()) {
			break
		}

		// flush any available messages
		flush()
	}
	logger.DebugContext(ctx, "closing outgoing")
	close(outGoingMessages)

	waitComplete := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitComplete)
	}()

	select {
	case <-waitComplete:
		return lastError
	case <-ctx.Done():
		return errors.Join(lastError, ctx.Err())
	}
}
