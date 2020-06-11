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
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}
