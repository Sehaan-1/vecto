package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "build" {
		_ = os.MkdirAll("dist", 0755)
		_ = os.WriteFile("dist/api_service.bin", []byte("compiled api service binary"), 0755)
		fmt.Println("API service binary built in dist/api_service.bin")
		return
	}
	fmt.Println("API service running")
}
