package internal

import (
	"fmt"
	"log/slog"

	"github.com/spiffe/go-spiffe/v2/logger"
)

var _ logger.Logger = (*SPIFFESlogAdapter)(nil)

// SPIFFESlogAdapter allows a slog logger to be injected into the SPIFFE GO
// SDK.
type SPIFFESlogAdapter struct {
	slog *slog.Logger
}

func NewSPIFFESlogAdapter(slog *slog.Logger) SPIFFESlogAdapter {
	return SPIFFESlogAdapter{slog: slog}
}

func (w SPIFFESlogAdapter) Debugf(format string, args ...interface{}) {
	w.slog.Debug(fmt.Sprintf(format, args...))
}

func (w SPIFFESlogAdapter) Infof(format string, args ...interface{}) {
	w.slog.Info(fmt.Sprintf(format, args...))
}

func (w SPIFFESlogAdapter) Warnf(format string, args ...interface{}) {
	w.slog.Warn(fmt.Sprintf(format, args...))
}

func (w SPIFFESlogAdapter) Errorf(format string, args ...interface{}) {
	w.slog.Error(fmt.Sprintf(format, args...))
}
