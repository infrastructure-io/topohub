package debug

import (
	"os"
	"path/filepath"
	"runtime"

	pyroscope "github.com/grafana/pyroscope-go"
	"github.com/infrastructure-io/topohub/pkg/log"
)

func RunPyroscope(serverAddress, pyroscopeTag string) {
	if serverAddress != "" {
		// push mode ,  push to pyroscope server
		log.Logger.Infof("pyroscope works in push mode, server %s, hostname %s ", serverAddress, pyroscopeTag)

		// These 2 lines are only required if you're using mutex or block profiling
		runtime.SetMutexProfileFraction(5)
		runtime.SetBlockProfileRate(5)

		_, e := pyroscope.Start(pyroscope.Config{
			ApplicationName: filepath.Base(os.Args[0]),
			ServerAddress:   serverAddress,
			// too much log
			// Logger:          pyroscope.StandardLogger,
			Logger: nil,
			Tags:   map[string]string{"node": pyroscopeTag},
			ProfileTypes: []pyroscope.ProfileType{
				pyroscope.ProfileCPU,
				pyroscope.ProfileInuseObjects,
				pyroscope.ProfileAllocObjects,
				pyroscope.ProfileInuseSpace,
				pyroscope.ProfileAllocSpace,
				pyroscope.ProfileGoroutines,
				pyroscope.ProfileMutexCount,
				pyroscope.ProfileMutexDuration,
				pyroscope.ProfileBlockCount,
				pyroscope.ProfileBlockDuration,
			},
		})
		if e != nil {
			log.Logger.Errorf("failed to setup pyroscope, reason=%v", e)
		}
	}
}
