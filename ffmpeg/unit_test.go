package ffmpeg

import (
	"VideoBatchCut/util"
	"log"
	"runtime"
	"testing"
)

func init() {
	util.SetLog("BitchCut.log")
	log.SetFlags(2 | 16)
}

func TestHasCUDA(t *testing.T) {
	HasH264NVENC()
}
func TestNumThreads(t *testing.T) {
	result := runtime.NumCPU()
	t.Logf("result:%d", result)
}
