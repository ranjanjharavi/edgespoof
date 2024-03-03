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

	domain := os.Args[1]
	cnames, terminal, err := resolver.TraceChain(context.Background(), domain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Input Domain   : %s\n", domain)
	fmt.Printf("CNAME Chain    : %v\n", cnames)
	fmt.Printf("Terminal Target: %s\n", terminal)
}
