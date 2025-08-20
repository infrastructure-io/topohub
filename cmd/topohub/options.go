package main

import "flag"

type FlagOptions struct {
	probePort        string
	webhookPort      string
	metricsPort      string
	pyroscopeAddress string
	pyroscopeTag     string
	pprofAddress     string
	pprofPort        string
}

func parseFlags(opts *FlagOptions) {
	// Parse command line flags
	flag.StringVar(&opts.probePort, "health-probe-port", "8081", "The address the probe endpoint binds to.")
	flag.StringVar(&opts.webhookPort, "webhook-port", "8082", "The address the probe endpoint binds to.")
	flag.StringVar(&opts.metricsPort, "metrics-port", "8083", "The address the metric endpoint binds to.")
	flag.StringVar(&opts.pyroscopeAddress, "pyroscope-address", "", "The server address where the pyroscope data is pushed.")
	flag.StringVar(&opts.pyroscopeTag, "pyroscope-tag", "", "The tag used for pyroscope.")
	flag.StringVar(&opts.pprofAddress, "pprof-address", "", "The address the pprof endpoint binds to.")
	flag.StringVar(&opts.pprofPort, "pprof-port", "", "The port used for pprof")
	flag.Parse()
}
