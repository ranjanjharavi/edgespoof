package main

import "github.com/ranjanjharavi/edgespoof/internal/system"

func reportHostsUpdate(message string, customHosts bool) {
	printSuccess(message)
	if customHosts {
		return
	}

	if err := system.FlushDNSCache(); err != nil {
		printFlushWarning(err)
		return
	}

	printSuccess("Flushed OS DNS cache")
}
