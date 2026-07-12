package nats

import (
	"fmt"
	"time"

	natsgo "github.com/nats-io/nats.go"
)

// Connect establishes the initial NATS connection synchronously. Once the
// initial connection succeeds, the client keeps retrying reconnects forever.
// This distinction lets production startup fail fast instead of reporting
// success while the client is only in a reconnect loop.
func Connect(url, token string) (*natsgo.Conn, error) {
	options := []natsgo.Option{
		natsgo.Name("hubvas"),
		natsgo.Timeout(5 * time.Second),
		natsgo.MaxReconnects(-1),
		natsgo.ReconnectWait(2 * time.Second),
	}
	if token != "" {
		options = append(options, natsgo.Token(token))
	}
	conn, err := natsgo.Connect(url, options...)
	if err != nil {
		return nil, err
	}
	if err := conn.FlushTimeout(5 * time.Second); err != nil {
		conn.Close()
		return nil, fmt.Errorf("verify NATS connection: %w", err)
	}
	return conn, nil
}
