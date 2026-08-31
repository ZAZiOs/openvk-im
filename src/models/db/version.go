package db_models

import (
	"fmt"
	"strconv"
	"strings"
)

type ApiV struct {
	Raw   string `json:"raw"`
	Major int    `json:"major"`
	Minor int    `json:"minor"`
}

func ParseApiV(vStr string) ApiV {
	vStr = strings.TrimSpace(vStr)
	if vStr == "" {
		vStr = "5.9999" // same as in ovk
	}

	parts := strings.Split(vStr, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil || major == 0 {
		major = 5
	}

	minor := 9999
	if len(parts) > 1 {
		if m, err := strconv.Atoi(parts[1]); err == nil {
			if len(parts[1]) == 2 {
				m = m * 10
			} else if len(parts[1]) == 1 {
				m = m * 100
			}
			minor = m
		}
	}

	return ApiV{
		Raw:   vStr,
		Major: major,
		Minor: minor,
	}
}

// IsOlderThan returns true if the current version is strictly older than major.minor.
// Supports both 2-digit (e.g. 5, 80) and 3-digit (e.g. 5, 800) target minor versions.
func (v ApiV) IsOlderThan(major, minor int) bool {
	if v.Major < major {
		return true
	}
	if v.Major > major {
		return false
	}

	targetMinor := minor
	if targetMinor < 100 && targetMinor > 0 {
		targetMinor = targetMinor * 10
	}

	return v.Minor < targetMinor
}

// IsAtLeast returns true if the current version is at least major.minor (>= major.minor).
func (v ApiV) IsAtLeast(major, minor int) bool {
	return !v.IsOlderThan(major, minor)
}

// String returns the raw version string.
func (v ApiV) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}
