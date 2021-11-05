package utils

// ListContainsString just searches if the val is in string list
func ListContainsString(list []string, val string) bool {
	for _, el := range list {
		if el == val {
			return true
		}
	}
	return false
}
