package main

import (
	"testing"
	"time"
)

func TestCanDownloadHonoursLearnerDeadline(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		due  time.Time
		want bool
	}{
		{name: "before deadline", due: now.Add(time.Minute), want: true},
		{name: "after deadline", due: now.Add(-time.Minute), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := canDownload(now, tc.due); got != tc.want {
				t.Fatalf("canDownload() = %v, want %v", got, tc.want)
			}
		})
	}
}
