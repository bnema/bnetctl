package wine

import (
	"regexp"
	"strconv"
)

var wineVersionPattern = regexp.MustCompile(`^wine-([0-9]+)\.([0-9]+)(?:$|[-. ])`)

func supportsWaylandHDR(version string) bool {
	parts := wineVersionPattern.FindStringSubmatch(version)
	if parts == nil {
		return false
	}
	major, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(parts[2])
	if err != nil {
		return false
	}
	return major > 11 || major == 11 && minor >= 19
}

// Keep the legacy preference unless a candidate meets the HDR minimum.
func selectHDRWine(preferred, pathWine string, version func(string) string) string {
	if supportsWaylandHDR(version(preferred)) {
		return preferred
	}
	if pathWine != "" && pathWine != preferred && supportsWaylandHDR(version(pathWine)) {
		return pathWine
	}
	return preferred
}
