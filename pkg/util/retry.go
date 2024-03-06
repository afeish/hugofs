package util

import (
	"strings"
	"time"

	"github.com/pingcap/log"
	"go.uber.org/zap"
)

var retryInterval = 6 * time.Second

func SetRetryInterval(interval time.Duration) {
	retryInterval = interval
}

func Retry(name string, job func() error) (err error) {
	waitTime := time.Second
	for waitTime < retryInterval {
		if err = job(); err != nil {
			if !strings.Contains(err.Error(), "transport") { // something bad happened
				return err
			}
			// transport error, do retry
			log.Info("retry", zap.String("name", name), zap.Error(err))
			time.Sleep(waitTime)
			waitTime += waitTime
			continue
		}
		return nil
	}
	return err
}
func RetryForever(name string, job func() error, onErrFn func(err error) (shouldContinue bool)) {
	waitTime := time.Second
	for {
		err := job()
		if err == nil {
			waitTime = time.Second
			break
		}
		if onErrFn(err) {
			if strings.Contains(err.Error(), "transport") {
				log.Error("retry", zap.String("name", name), zap.Error(err))
			}
			time.Sleep(waitTime)
			if waitTime < retryInterval {
				waitTime += waitTime / 2
			}
			continue
		}
	}
}
