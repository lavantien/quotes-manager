package bilara

import (
	"regexp"
	"strconv"
)

var rangeMuidRE = regexp.MustCompile(`^(.+?)(\d+)-(\d+)$`)

func AllowedMuids(fileMuid string, maxExpansion int) map[string]bool {
	out := map[string]bool{fileMuid: true}
	m := rangeMuidRE.FindStringSubmatch(fileMuid)
	if m == nil {
		return out
	}
	start, errStart := strconv.Atoi(m[2])
	end, errEnd := strconv.Atoi(m[3])
	if errStart != nil || errEnd != nil || end < start || end-start+1 > maxExpansion {
		return out
	}
	for i := start; i <= end; i++ {
		out[m[1]+strconv.Itoa(i)] = true
	}
	return out
}
