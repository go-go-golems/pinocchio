package profiles

import "strings"

const MockParityProfile = "mock_parity"

func IsMockParityProfile(profile string) bool {
	return strings.EqualFold(strings.TrimSpace(profile), MockParityProfile)
}
