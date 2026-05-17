package core

import (
	"VideoBatchCut/util"
	"testing"
)

func TestDJI(t *testing.T) {
	util.SetLog("dji.log")
	t.Log("DJI test")
	DJI("D:\\dji", "D:\\dji\\20.mp3")
}
