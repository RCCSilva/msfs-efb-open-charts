package server

import "mime"

// The browser refuses a module script that is not served as JavaScript. Go asks the registry of Windows for the type of
// a file extension, and a machine whose registry says text/plain for .js would break the whole manager, so it is set here.
func init() {
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
}
