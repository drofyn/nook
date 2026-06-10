package main

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed web/*
var webFS embed.FS

var webRoot fs.FS

func init() {
	var err error
	webRoot, err = fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
}
