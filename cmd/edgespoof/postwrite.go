package main

import (
	"fmt"
	"os"

	"github.com/yupcode/edgespoof/internal/system"
)

func reportHostsUpdate(message string, customHosts bool) {
	fmt.Printf("%s✔%s %s\n", colorGreen, colorReset, message)
	if customHosts {
		return
	}

	if err := system.FlushDNSCache(); err != nil {
		fmt.Fprintf(os.Stderr, "%s! Warning:%s Hosts file updated, but OS DNS cache flush failed: %v\n", colorYellow, colorReset, err)
		return
	}
	
	fmt.Printf("%s✔%s Flushed OS DNS cache\n", colorGreen, colorReset)
}
