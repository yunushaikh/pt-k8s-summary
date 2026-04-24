package k8sfmt

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var quantityRe = regexp.MustCompile(`^([0-9]*\.?[0-9]+)([A-Za-z]*)$`)

// ToBytes parses a Kubernetes resource quantity (e.g. "8Gi", "100Mi") into bytes.
func ToBytes(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	m := quantityRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	suf := m[2]
	if suf == "" {
		return val, true
	}
	mult := 1.0
	switch suf {
	case "n":
		mult = 1e-9
	case "u":
		mult = 1e-6
	case "m":
		mult = 1e-3
	case "k":
		mult = 1e3
	case "M":
		mult = 1e6
	case "G":
		mult = 1e9
	case "T":
		mult = 1e12
	case "P":
		mult = 1e15
	case "E":
		mult = 1e18
	case "Ki":
		mult = 1024
	case "Mi":
		mult = 1024 * 1024
	case "Gi":
		mult = 1024 * 1024 * 1024
	case "Ti":
		mult = 1024 * 1024 * 1024 * 1024
	case "Pi":
		mult = 1024 * 1024 * 1024 * 1024 * 1024
	case "Ei":
		mult = 1024 * 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return 0, false
	}
	return val * mult, true
}

// HumanSize renders bytes as GiB / MiB / KiB / B.
func HumanSize(b float64) string {
	if b <= 0 {
		return "—"
	}
	if b >= 1024*1024*1024 {
		return fmt.Sprintf("%.2f GiB", b/(1024*1024*1024))
	}
	if b >= 1024*1024 {
		return fmt.Sprintf("%.2f MiB", b/(1024*1024))
	}
	if b >= 1024 {
		return fmt.Sprintf("%.2f KiB", b/1024)
	}
	return fmt.Sprintf("%.0f B", b)
}

// HumanQuantity parses a Kubernetes quantity string and formats it for display.
func HumanQuantity(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	b, ok := ToBytes(s)
	if !ok {
		return s
	}
	return HumanSize(b)
}
