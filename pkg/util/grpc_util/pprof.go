package grpc_util

import (
	"net/http"
	"runtime"

	_ "net/http/pprof"

	"github.com/afeish/hugo/global"
	"github.com/maruel/panicparse/stack/webstack"
	"github.com/pingcap/log"
	"github.com/pyroscope-io/client/pyroscope"
	"go.uber.org/zap"
)

func StartPyroscopeServerAddr(applicationName, serveAddr string) {
	if serveAddr == "" {
		log.Info("pyroscope server addr is empty, skip")
		return
	}
	if applicationName == "" {
		applicationName = global.GetEnvCfg().Tracing.Host
	}

	// These 2 lines are only required if you're using mutex or block profiling
	// Read the explanation below for how to set these rates:
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	log.Info("starting pprof server on:", zap.String("application", applicationName), zap.String("pyroscopeServerAddr", serveAddr))
	if _, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: applicationName,
		ServerAddress:   serveAddr,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	}); err != nil {
		log.Error("failed to start pyroscope: %v", zap.Error(err))
	}
}

func StartPprof(serveAddr string) {
	if serveAddr == "" {
		log.Info("pprof server addr is empty, skip")
		return
	}
	// start pprof by default
	log.Info("starting pprof server on:", zap.String("pprofServer", serveAddr))
	http.HandleFunc("/debug/panicparse", webstack.SnapshotHandler)
	if err := http.ListenAndServe(serveAddr, nil); err != nil {
		log.Fatal("cannot start pprof server at %v", zap.String("pprofServer", serveAddr), zap.Error(err))
	}
}
