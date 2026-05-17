package core

import (
	"VideoBatchCut/util"
	"testing"
)
// go test -v -timeout 0 -run TestDJI
func TestDJI(t *testing.T) {
	util.SetLog("dji.log")
	t.Log("DJI test")
	DJI("D:\\dji", "D:\\dji\\20.mp3")
}
