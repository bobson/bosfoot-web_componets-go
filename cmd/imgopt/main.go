package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := "public/images"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".png")) {
			fmt.Printf("Optimizing: %s\n", path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
