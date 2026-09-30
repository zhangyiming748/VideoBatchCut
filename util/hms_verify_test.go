package util

import "testing"

func TestFormatSecondToHMS(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "00:00:00.000"},
		{1507.5520733265307, "00:25:07.552"},  // .llc 真实值，截断而非进位
		{59.9996, "00:00:59.999"},             // 旧实现 bug 触发点：曾输出 "00:00:59.10"
		{3599.9999, "00:59:59.999"},           // 旧实现会截成 "00:59:59.10"
		{4658.975435, "01:17:38.975"},
		{-1, "00:00:00.000"},
	}
	for _, c := range cases {
		if got := FormatSecondToHMS(c.in); got != c.want {
			t.Errorf("FormatSecondToHMS(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
