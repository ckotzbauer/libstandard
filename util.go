package libstandard

import "strings"

// Unescape removes backslashes and double-quotes from strings
func Unescape(s string) string {
	s = strings.ReplaceAll(s, "\\", "")
	s = strings.ReplaceAll(s, "\"", "")
	return s
}

// Unique removes all duplicate values from the given slice
func Unique(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// FirstOrEmpty returns the first string from the slice.
func FirstOrEmpty(slice []string) string {
	if len(slice) > 0 {
		return slice[0]
	}

	return ""
}

// ToMap converts a string-slice to a map[string]string
func ToMap(slice []string) map[string]string {
	m := make(map[string]string, 0)

	for _, s := range slice {
		if len(strings.TrimSpace(s)) == 0 {
			continue
		}

		splitted := strings.Split(s, "=")
		m[splitted[0]] = splitted[1]
	}

	return m
}
