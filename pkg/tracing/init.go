package tracing

import (
	"context"
	"os"
	"sync"

	"github.com/pingcap/log"
	"go.uber.org/zap"
)

type shutFunc func(ctx context.Context) error

var (
	tracingProvider *Provider
	shut            shutFunc
	once            sync.Once
)

func Init(serviceName string) error {
	var err error
	once.Do(func() {
		shut = func(ctx context.Context) error { return nil }
		backendURL := os.Getenv("TRACING_BACKEND_URL")
		if backendURL == "" {
			return
		}
		log.Debug("init tracing backend provider", zap.String("backend-url", backendURL))
		// init tracer
		tracingProvider, err = NewProvider(
			context.Background(), serviceName, backendURL)
		if err != nil {
			log.Warn("not trace provider", zap.Error(err))
			return
		}
		// register tracing provider as a global provider
		shut, err = tracingProvider.RegisterAsGlobal()
		if err != nil {
			log.Error("RegisterAsGlobal", zap.Error(err))
		}
	})
	return err
}

func Destory(ctx context.Context) error {
	if shut == nil {
		return nil
	}
	if err := shut(ctx); err != nil {
		log.Error("shutdown provider", zap.Error(err))
		return err
	}
	log.Debug("destroy tracing provider")
	return nil
}
