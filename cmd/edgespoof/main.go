package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yupcode/edgespoof/internal/resolver"
)

func main() {
	v4Only := flag.Bool("v4-only", false, "Resolve IPv4 (A) record only")
	staging := flag.Bool("staging", false, "Rewrite CDN CNAME to staging network")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: go run ./cmd/edgespoof [--v4-only] <domain>")
		os.Exit(1)
	}

	domainInput := flag.Arg(0)
	startTime := time.Now()

	res, err := resolver.Resolve(context.Background(), domainInput, *staging, *v4Only)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Domain       : %s\n", res.Domain)
	fmt.Printf("CNAME Chain  : %v\n", res.CNAMEChain)
	if res.StagingSwap != "" {
		fmt.Printf("Staging Swap : %s\n", res.StagingSwap)
	}
	fmt.Printf("Target Edge  : %s\n", res.TargetCNAME)
	fmt.Printf("First IPv4   : %s\n", res.IPv4)
	fmt.Printf("First IPv6   : %s\n", res.IPv6)
	fmt.Printf("Resolved in  : %v\n", time.Since(startTime).Round(time.Millisecond))
}
