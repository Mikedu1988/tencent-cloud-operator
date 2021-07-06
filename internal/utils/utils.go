package utils

// Contains verifies if string slice contains a string
func Contains(l []string, s string) bool {
	for _, str := range l {
		if str == s {
			return true
		}
	}
	return false
}
