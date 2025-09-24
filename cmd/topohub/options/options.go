package options

import "flag"

type TopohubFlags struct {
	WebhookPort      string
	MetricsPort      string
	PyroscopeAddress string
	PyroscopeTag     string
	PprofAddress     string
	PprofPort        string
}

func ParseFlags(opts *TopohubFlags) {
	// Parse command line flags
	flag.StringVar(&opts.WebhookPort, "webhook-port", "8082", "The address the probe endpoint binds to.")
	flag.StringVar(&opts.MetricsPort, "metrics-port", "8083", "The address the metric endpoint binds to.")
	flag.StringVar(&opts.PyroscopeAddress, "pyroscope-address", "", "The server address where the pyroscope data is pushed.")
	flag.StringVar(&opts.PyroscopeTag, "pyroscope-tag", "", "The tag used for pyroscope.")
	flag.StringVar(&opts.PprofAddress, "pprof-address", "", "The address the pprof endpoint binds to.")
	flag.StringVar(&opts.PprofPort, "pprof-port", "", "The port used for pprof")
	flag.Parse()
}
