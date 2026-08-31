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
func (v ApiV) IsOlderThan(major, minor int) bool {
	if v.Major < major {
		return true
	}
	if v.Major == major && v.Minor < minor {
		return true
	}
	return false
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
