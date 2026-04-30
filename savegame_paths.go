package main

import "strings"

func sanitizeSaveFileComponent(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "save"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if prevDash {
			continue
		}
		b.WriteByte('-')
		prevDash = true
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "save"
	}
	return out
}
