package closers

import (
	"io"
	"log/slog"
)

func CloseOrLog(c io.Closer, log *slog.Logger) {
	if err := c.Close(); err != nil {
		log.Error("close failed", "error", err)
	}
}

func CloseOrPanic(c io.Closer) {
	if err := c.Close(); err != nil {
		panic("close failed: " + err.Error())
	}
}
