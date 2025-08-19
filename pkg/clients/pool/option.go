package pool

import (
	"go.uber.org/zap"

	"github.com/infrastructure-io/topohub/pkg/log"
)

const (
	DefaultMaxIdleHour    int32 = 24
	DefaultGcIntervalHour int32 = 6
)

type Options struct {
	maxIdleHour    int32
	gcIntervalHour int32
	logger         *zap.SugaredLogger
}

func buildOptions(opts ...Option) Options {
	var o Options
	for _, opt := range opts {
		opt(&o)
	}
	if o.maxIdleHour == 0 {
		o.maxIdleHour = DefaultMaxIdleHour
	}
	if o.gcIntervalHour == 0 {
		o.gcIntervalHour = DefaultGcIntervalHour
	}
	if o.logger == nil {
		o.logger = log.Logger.Named("sessionPool")
	}
	return o
}

type Option func(*Options)

func WithMaxIdleHouer(maxIdleHour int32) Option {
	return func(opts *Options) {
		opts.maxIdleHour = maxIdleHour
	}
}

func WithGcIntervalHour(gcIntervalHour int32) Option {
	return func(opts *Options) {
		opts.gcIntervalHour = gcIntervalHour
	}
}

func WithLogger(logger *zap.SugaredLogger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}
