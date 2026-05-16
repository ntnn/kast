package resources

import (
	"regexp"
	"strings"
)

// rfc1123Re matches characters not allowed in an RFC 1123.
var rfc1123Re = regexp.MustCompile(`[^a-z0-9-]+`)

// RFC1123Label turns the given string into a valid RFC 1123 name so it
// can be used as either a kube resource or an applyset name.
func RFC1123Label(name string) string {
	name = strings.ToLower(name)
	name = rfc1123Re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "-")
	}
	return name
}
