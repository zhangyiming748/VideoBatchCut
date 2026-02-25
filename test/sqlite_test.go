package t_test

import (
	"testing"

	"VideoBatchCut/sqlite"
)

func TestSqlite(t *testing.T) {
	sqlite.SetSqlite()
	bc := new(sqlite.BatchCut)
	bc.Sync()
	bc.Index = "1"
	bc.FileName = "1.mp4"
	bc.Start = "1"
	bc.End = "2"
	bc.Insert()
}
