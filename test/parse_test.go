package t_test

import (
	"fmt"
	"testing"

	"VideoBatchCut/util"
)

func TestParse(t *testing.T) {
	segments, err := util.ParseSegments("proj.llc")
	if err != nil {
		t.Errorf("解析失败: %v", err)
		return
	}

	// 打印解析结果，只输出存在的字段
	for i, seg := range segments {
		fmt.Printf("Segment %d:\n", i+1)
		if seg.Start != 0 {
			fmt.Printf("  Start: %f\n", seg.Start)
		}
		if seg.End != 0 {
			fmt.Printf("  End: %f\n", seg.End)
		}
		if seg.Name != "" {
			fmt.Printf("  Name: %s\n", seg.Name)
		}
	}
}
