package importer

import "strings"

var protectedFieldFragments = []string{
	".secrets.",
	".secret",
	".password",
	".private_key",
	".privatekey",
	".access_key",
	".secret_access_key",
	".client_secret",
	".token",
	".credential",
}

func IsProtectedField(path string) bool {
	normalized := "." + strings.ToLower(strings.TrimSpace(path)) + "."
	for _, fragment := range protectedFieldFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}
