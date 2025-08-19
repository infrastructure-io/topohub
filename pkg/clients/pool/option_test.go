package pool

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/infrastructure-io/topohub/pkg/log"
)

func TestBuildOptions(t *testing.T) {
	t.Run("default value", func(t *testing.T) {
		opts := buildOptions()
		assert.Equal(t, DefaultMaxIdleHour, opts.maxIdleHour)
		assert.Equal(t, DefaultGcIntervalHour, opts.gcIntervalHour)
		assert.Equal(t, "sessionPool", opts.logger.Desugar().Name())
	})

	t.Run("custom value", func(t *testing.T) {
		opts := buildOptions(
			WithMaxIdleHouer(5),
			WithGcIntervalHour(1),
			WithLogger(log.Logger.Named("custom")),
		)
		assert.Equal(t, int32(5), opts.maxIdleHour)
		assert.Equal(t, int32(1), opts.gcIntervalHour)
		assert.Equal(t, "custom", opts.logger.Desugar().Name())
	})
}
