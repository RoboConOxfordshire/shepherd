package main

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed web/editor web/static web/docs
var webFS embed.FS

var staticFS fs.FS

func init() {
	var err error
	staticFS, err = fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("Failed to get web subfs: %v", err)
	}
}
