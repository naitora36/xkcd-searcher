package broker

import (
	"log/slog"

	"github.com/nats-io/nats.go"
)

const XKCDUpdateTopic = "xkcd.db.updated"

type Broker struct {
	Conn *nats.Conn
	log  *slog.Logger
}

func NewBroker(address string, log *slog.Logger) (*Broker, error) {
	nc, err := nats.Connect(address)
	if err != nil {
		log.Error("connection problem", "address", address, "error", err)
		return nil, err
	}

	return &Broker{
		Conn: nc,
		log:  log,
	}, nil
}

func (b *Broker) SendEvent(event string) {
	err := b.Conn.Publish(XKCDUpdateTopic, []byte(event))
	if err != nil {
		b.log.Error("could not publish message", "error", err)
	}

	err = b.Conn.Flush()
	if err != nil {
		b.log.Error("problem when flush message", "error", err)
	}
}

func (b *Broker) Close() error {
	err := b.Conn.Drain()
	if err != nil {
		return err
	}
	return nil
}
