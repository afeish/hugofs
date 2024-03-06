package global

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pingcap/log"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	logFileNamePrefix = "hugo-"
	logFileNameSuffix = ".log"
	logsDirMode       = 0o700
)

var (
	logger     *zap.Logger
	loggerOnce sync.Once
)

func GetLogger() *zap.Logger {
	loggerOnce.Do(func() {
		cfg := GetEnvCfg()

		logLevel := cfg.Log.Level
		if cfg.Test {
			logLevel = "debug"
		}

		loggerSetting := loggerSetting{
			jsonLogConsole:       false,
			consoleLogTimestamps: true,
			disableColor:         false,

			test:                  cfg.Test,
			logLevel:              logLevel,
			logDir:                cfg.Log.Dir,
			logDirMaxFiles:        10,
			logDirMaxTotalSizeMB:  100,
			logFileMaxSegmentSize: 10 * 1024 * 1024,
			logDirMaxAge:          time.Hour * 24,
		}

		cores := []zapcore.Core{loggerSetting.setupConsoleCore()}
		if cfg.Log.FileEnabled && !cfg.Test {
			cores = append(cores, loggerSetting.setupLogFileCore(time.Now(), "hugo"))
		}

		core := zapcore.NewTee(
			cores...)
		logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	})
	return logger
}

type loggerSetting struct {
	jsonLogConsole       bool
	consoleLogTimestamps bool
	disableColor         bool
	logLevel             string

	test                  bool
	logDir                string
	logFile               string
	logFileMaxSegmentSize int
	logDirMaxFiles        int
	logDirMaxTotalSizeMB  float64
	logDirMaxAge          time.Duration
}

func (l *loggerSetting) setupConsoleCore() zapcore.Core {
	config := zap.NewProductionEncoderConfig()
	if l.test {
		config = zap.NewDevelopmentEncoderConfig()
	}
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(config)

	return zapcore.NewCore(
		consoleEncoder, zapcore.AddSync(os.Stdout),
		logLevelFromFlag(strings.ToLower(l.logLevel)),
	)
}

func (l *loggerSetting) setupLogFileCore(now time.Time, suffix string) zapcore.Core {
	config := zap.NewProductionEncoderConfig()
	if l.test {
		config = zap.NewDevelopmentEncoderConfig()
	}
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	fileEncoder := zapcore.NewJSONEncoder(config)
	return zapcore.NewCore(
		fileEncoder,
		l.setupLogFileBasedLogger(time.Now(), "cli-logs", suffix, l.logFile, l.logDirMaxFiles, l.logDirMaxTotalSizeMB, l.logDirMaxAge),
		logLevelFromFlag(strings.ToLower(l.logLevel)),
	)
}

func (l *loggerSetting) setupLogFileBasedLogger(now time.Time, subdir, suffix, logFileOverride string, maxFiles int, maxSizeMB float64, maxAge time.Duration) zapcore.WriteSyncer {
	var logFileName, symlinkName string

	if logFileOverride != "" {
		var err error

		logFileName, err = filepath.Abs(logFileOverride)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to resolve logs path", err)
		}
	}

	if logFileName == "" {
		logBaseName := fmt.Sprintf("%v%v-%v-%v%v", logFileNamePrefix, now.Format("20060102-150405"), os.Getpid(), suffix, logFileNameSuffix)
		logFileName = filepath.Join(l.logDir, subdir, logBaseName)
		symlinkName = "latest.log"
	}

	logDir := filepath.Dir(logFileName)
	logFileBaseName := filepath.Base(logFileName)

	if err := os.MkdirAll(logDir, logsDirMode); err != nil {
		fmt.Fprintln(os.Stderr, "Unable to create logs directory:", err)
	}

	sweepLogWG := &sync.WaitGroup{}
	doSweep := func() {}

	// do not scrub directory if custom log file has been provided.
	if logFileOverride == "" && shouldSweepLog(maxFiles, maxAge) {
		doSweep = func() {
			sweepLogDir(context.TODO(), logDir, maxFiles, maxSizeMB, maxAge)
		}
	}

	odf := &onDemandFile{
		logDir:          logDir,
		logFileBaseName: logFileBaseName,
		symlinkName:     symlinkName,
		maxSegmentSize:  l.logFileMaxSegmentSize,
		startSweep: func() {
			sweepLogWG.Add(1)

			go func() {
				defer sweepLogWG.Done()

				doSweep()
			}()
		},
	}

	// old behavior: start log sweep in parallel to program but don't wait at the end.
	odf.startSweep()
	return odf

}

func shouldSweepLog(maxFiles int, maxAge time.Duration) bool {
	return maxFiles > 0 || maxAge > 0
}

func sweepLogDir(ctx context.Context, dirname string, maxCount int, maxSizeMB float64, maxAge time.Duration) {
	var timeCutoff time.Time
	if maxAge > 0 {
		timeCutoff = time.Now().Add(-maxAge)
	}

	if maxCount == 0 {
		maxCount = math.MaxInt32
	}

	maxTotalSizeBytes := int64(maxSizeMB * 1e6)

	entries, err := os.ReadDir(dirname)
	if err != nil {
		log.Error("unable to read log directory", zap.Error(err))
		return
	}

	fileInfos := make([]os.FileInfo, 0, len(entries))

	for _, e := range entries {
		info, err2 := e.Info()
		if os.IsNotExist(err2) {
			// we lost the race, the file was deleted since it was listed.
			continue
		}

		if err2 != nil {
			log.Error("unable to read file info", zap.Error(err2))
			return
		}

		fileInfos = append(fileInfos, info)
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].ModTime().After(fileInfos[j].ModTime())
	})

	cnt := 0
	totalSize := int64(0)

	for _, fi := range fileInfos {
		if !strings.HasPrefix(fi.Name(), logFileNamePrefix) {
			continue
		}

		if !strings.HasSuffix(fi.Name(), logFileNameSuffix) {
			continue
		}

		cnt++

		totalSize += fi.Size()

		if cnt > maxCount || totalSize > maxTotalSizeBytes || fi.ModTime().Before(timeCutoff) {
			if err = os.Remove(filepath.Join(dirname, fi.Name())); err != nil && !os.IsNotExist(err) {
				log.Error("unable to remove log file", zap.Error(err))
			}
		}
	}
}

func logLevelFromFlag(levelString string) zapcore.LevelEnabler {
	switch levelString {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warning":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "dpanic":
		return zap.DPanicLevel
	case "panic":
		return zap.PanicLevel
	default:
		return zap.InfoLevel
	}
}

type onDemandFile struct {
	// +checklocks:mu
	segmentCounter int // number of segments written

	// +checklocks:mu
	currentSegmentSize int // number of bytes written to current segment

	// +checklocks:mu
	maxSegmentSize int

	// +checklocks:mu
	currentSegmentFilename string

	// +checklocks:mu
	logDir string

	// +checklocks:mu
	logFileBaseName string

	// +checklocks:mu
	symlinkName string

	startSweep func()

	mu sync.Mutex
	f  *os.File
}

func (w *onDemandFile) Sync() error {
	if w.f == nil {
		return nil
	}

	//nolint:wrapcheck
	return w.f.Sync()
}

func (w *onDemandFile) closeSegmentAndSweep() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.closeSegmentAndSweepLocked()
}

func (w *onDemandFile) closeSegmentAndSweepLocked() {
	if w.f != nil {
		if err := w.f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: unable to close log segment: %v", err)
		}

		w.f = nil
	}

	w.startSweep()
}

func (w *onDemandFile) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// close current file if we'd overflow on next write.
	if w.f != nil && w.currentSegmentSize+len(b) > w.maxSegmentSize {
		w.closeSegmentAndSweepLocked()
	}

	// open file if we don't have it yet
	if w.f == nil {
		var baseName, ext string

		p := strings.LastIndex(w.logFileBaseName, ".")
		if p < 0 {
			ext = ""
			baseName = w.logFileBaseName
		} else {
			ext = w.logFileBaseName[p:]
			baseName = w.logFileBaseName[0:p]
		}

		w.currentSegmentFilename = fmt.Sprintf("%s.%d%s", baseName, w.segmentCounter, ext)
		w.segmentCounter++
		w.currentSegmentSize = 0

		lf := filepath.Join(w.logDir, w.currentSegmentFilename)

		f, err := os.Create(lf) //nolint:gosec
		if err != nil {
			return 0, errors.Wrap(err, "unable to open log file")
		}

		w.f = f

		if w.symlinkName != "" {
			symlink := filepath.Join(w.logDir, w.symlinkName)
			_ = os.Remove(symlink)                            // best-effort remove
			_ = os.Symlink(w.currentSegmentFilename, symlink) // best-effort symlink
		}
	}

	n, err := w.f.Write(b)
	w.currentSegmentSize += n

	//nolint:wrapcheck
	return n, err
}
