package sensor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func Test_makeHostDirFilesInfo(t *testing.T) {
	hostFileSystemDefaultLocation = "."
	fileInfos, err := makeHostDirFilesInfo("testdata/testmakehostfiles", true, nil, 0)
	assert.NoError(t, err)
	assert.Len(t, fileInfos, 5)

	// Test maxRecursionDepth
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)
	zap.ReplaceGlobals(observedLogger)

	fileInfos, err = makeHostDirFilesInfo("testdata/testmakehostfiles", true, nil, maxRecursionDepth-1)
	assert.NoError(t, err)
	assert.Len(t, fileInfos, 4)
	assert.Len(t, observedLogs.FilterMessage("max recusrion depth exceeded").All(), 1)
}
