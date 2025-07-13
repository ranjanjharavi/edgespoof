package main

import "runtime/debug"

// Go stamps the module version at build time: tag, pseudo-version, or (devel)
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "unknown"
	}
	return info.Main.Version
}
