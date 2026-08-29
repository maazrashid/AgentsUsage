package web

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var assets embed.FS

var Static = mustSub(assets, "static")

func mustSub(source fs.FS, dir string) fs.FS {
	result, err := fs.Sub(source, dir)
	if err != nil {
		panic(err)
	}
	return result
}
