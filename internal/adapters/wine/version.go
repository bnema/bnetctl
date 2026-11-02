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
