package chartfox

import "strings"

// Kind maps a ChartFox chart type (`type_key`) to the app's chart type code ("IAC", "SID"...), or "" when ChartFox
// does not say (type "Unknown"), so the chart goes through review like any file of an unknown kind.
func Kind(typeKey string) string {
	switch strings.ToLower(typeKey) {
	case "approach":
		return "IAC"
	case "sid":
		return "SID"
	case "star":
		return "STAR"
	case "groundlayout", "ground":
		return "ADC"
	}
	return ""
}
