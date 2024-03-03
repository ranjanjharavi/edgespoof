package main

import (
	"context"
	"fmt"
	"os"

	"github.com/yupcode/edgespoof/internal/resolver"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/edgespoof <domain>")
		os.Exit(1)
	}

	domainInput := os.Args[1]
	res, err := resolver.Resolve(context.Background(), domainInput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Domain      : %s\n", res.Domain)
	fmt.Printf("CNAME Chain : %v\n", res.CNAMEChain)
	fmt.Printf("Target Edge : %s\n", res.TargetCNAME)
	fmt.Printf("First IPv4  : %s\n", res.IPv4)
	fmt.Printf("First IPv6  : %s\n", res.IPv6)
}
