package initiator

import (
	"context"
	"log/slog"
	"time"

	"yadro.com/course/isearch/core"
)

type Initiator struct {
	Ticker  *time.Ticker
	service core.Searcher
}

func NewInitiator(duration time.Duration, service core.Searcher) *Initiator {
	return &Initiator{
		Ticker:  time.NewTicker(duration),
		service: service,
	}
}

func (i *Initiator) StopTicker() {
	i.Ticker.Stop()
}

// Ошибки связанные с построением индекса буду лишь логировать, дабы из-за кратковременной ошибки БД не останавливать таймер навсегда
func (i *Initiator) Tick(ctx context.Context, log *slog.Logger) {
	err := i.service.BuildIndex(ctx)
	if err != nil {
		log.Error("in first build index we have error", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("in timer context canceled ")
			return
		case <-i.Ticker.C:
			err := i.service.BuildIndex(ctx)
			if err != nil {
				log.Error("build index error", "error", err)
			}
		}
	}
}
