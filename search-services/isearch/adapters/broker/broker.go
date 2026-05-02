package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/nats-io/nats.go"
	"yadro.com/course/isearch/core"
)

const (
	XKCDUpdateTopic = "xkcd.db.updated"
	eventUpdate     = "XKCD DB has been updated"
	eventDrop       = "XKCD DB has been dropped"
	opUpdate        = "update"
	opDrop          = "drop"
)

type Broker struct {
	Conn    *nats.Conn
	service core.Searcher
	log     *slog.Logger
}

func NewBroker(address string, service core.Searcher, log *slog.Logger) (*Broker, error) {
	nc, err := nats.Connect(address)
	if err != nil {
		log.Error("connection problem", "address", address, "error", err)
		return nil, err
	}

	return &Broker{
		Conn:    nc,
		service: service,
		log:     log,
	}, nil
}

func (b *Broker) GetEvent(ctx context.Context) error {
	sub, err := b.Conn.SubscribeSync(XKCDUpdateTopic)
	if err != nil {
		return err
	}

	for {
		msg, err := sub.NextMsgWithContext(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				b.log.Info("subscriber stopped by context")
				break
			}

			b.log.Error("cannot get next message", "error", err)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second * 2):
				continue
			}
		}
		b.processMsg(ctx, msg.Data)
	}

	if err = sub.Unsubscribe(); err != nil {
		return err
	}

	return nil
}

func (b *Broker) processMsg(ctx context.Context, data []byte) {
	switch string(data) {
	case eventUpdate:
		err := withDurationMetric(opUpdate, func() error {
			return b.service.BuildIndex(ctx)
		})
		if err != nil {
			b.log.Error("build index error", "error", err)
		}
	case eventDrop:
		_ = withDurationMetric(opDrop, func() error {
			b.service.DropIndex()
			return nil
		})
	default:
		b.log.Warn("unknown event received", "data", string(data))
	}
}

func (b *Broker) Close() error {
	err := b.Conn.Drain()
	if err != nil {
		return err
	}
	return nil
}

func withDurationMetric(operationName string, fn func() error) error {
	status := "success"

	now := time.Now()

	err := fn()

	duration := time.Since(now).Seconds()

	if err != nil {
		status = "error"
	}

	metricName := fmt.Sprintf(`index_duration_second{name="%s", status="%s"}`, operationName, status)
	metrics.GetOrCreateHistogram(metricName).Update(duration)

	return err
}
