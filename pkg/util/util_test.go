package util

import (
	"testing"
	"time"
)

func TestDuration_Seconds(t *testing.T) {
	if got := Duration(2 * time.Second); got != "2s" {
		t.Errorf("expected 2s, got %q", got)
	}
}

func TestDuration_SubSecondTruncated(t *testing.T) {
	if got := Duration(500 * time.Millisecond); got != "0s" {
		t.Errorf("expected 0s, got %q", got)
	}
}

func TestDuration_ExactMinuteStripsZeroSeconds(t *testing.T) {
	if got := Duration(60 * time.Second); got != "1m" {
		t.Errorf("expected 1m, got %q", got)
	}
}

func TestDuration_MinutesAndSeconds(t *testing.T) {
	if got := Duration(90 * time.Second); got != "1m30s" {
		t.Errorf("expected 1m30s, got %q", got)
	}
}

func TestDuration_ExactHourStripsZeroMinutes(t *testing.T) {
	if got := Duration(3600 * time.Second); got != "1h" {
		t.Errorf("expected 1h, got %q", got)
	}
}

func TestDuration_HoursMinutesSeconds(t *testing.T) {
	if got := Duration(3661 * time.Second); got != "1h1m1s" {
		t.Errorf("expected 1h1m1s, got %q", got)
	}
}

func TestDuration_HoursAndSeconds(t *testing.T) {
	// 1h0m30s → strips "m0s" → "1h0m30s"... wait: "1h0m30s" does not end with "m0s"
	// Actually 1h30s = 3630s → "1h0m30s", HasSuffix("1h0m30s","m0s") false, HasSuffix("1h0m30s","h0m") false
	if got := Duration(3630 * time.Second); got != "1h0m30s" {
		t.Errorf("expected 1h0m30s, got %q", got)
	}
}
