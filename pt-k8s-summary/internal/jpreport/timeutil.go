package jpreport

import (
	"fmt"
	"time"
)

// HumanizeDurationInState formats duration from `from` to `to` in short form (used for pod/node ages).
func HumanizeDurationInState(from, to time.Time) string {
	if !to.After(from) {
		return "0s"
	}
	d := to.Sub(from)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Round(time.Second)/time.Second))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours() / 24)
	h := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, h)
}
