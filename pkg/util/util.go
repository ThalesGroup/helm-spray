package util

import (
	"strings"
	"time"
)

func Duration(d time.Duration) string {
	d = d.Truncate(time.Second)
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	s = strings.Replace(s, "h0m", "h", 1)
	return s
}
