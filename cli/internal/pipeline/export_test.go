// export_test.go exposes package-private symbols to the _test package for
// white-box testing. This file is only compiled when running tests.
package pipeline

import (
	"context"
	"os"
	"time"
)

// ErrDryRunDoneForTest returns the dry-run sentinel so test code can
// assert pipeline.IsDryRunDone() without importing the concrete type.
func ErrDryRunDoneForTest() error { return errDryRunDone }

// SanitizeLogSlugForTest exposes sanitizeLogSlug for unit testing.
func SanitizeLogSlugForTest(name string) string { return sanitizeLogSlug(name) }

// OpenEngineLogFileForTest exposes openEngineLogFile for unit testing.
func OpenEngineLogFileForTest(cacheDir, recipeName string) (*os.File, error) {
	return openEngineLogFile(cacheDir, recipeName)
}

// PruneEngineLogsForTest exposes pruneEngineLogs for unit testing.
func PruneEngineLogsForTest(logDir string, keep int) error {
	return pruneEngineLogs(logDir, keep)
}

// WaitForEngineReadyForTest exposes waitForEngineReady for unit testing.
func WaitForEngineReadyForTest(
	ctx context.Context,
	healthURL string,
	timeout time.Duration,
	logPath string,
	engineDone chan error,
) error {
	realEngineDone := make(chan engineResult, 1)
	go func() {
		if engineDone != nil {
			err, ok := <-engineDone
			if ok {
				realEngineDone <- engineResult{
					runErr: err,
				}
			}
		}
	}()
	return waitForEngineReady(ctx, healthURL, timeout, logPath, realEngineDone)
}
