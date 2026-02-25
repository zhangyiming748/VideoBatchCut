package stand

import (
	"log"
	"testing"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}

// go test -v -timeout 10h -run TestFastAVC
func TestFastAVC(t *testing.T) {
	FastAVC("I:\\Pikpak\\My Pack\\[TSDV-41433]新原里彩 Pure smile(ピュア・スマイル)\\VIDEO_TS")
}
