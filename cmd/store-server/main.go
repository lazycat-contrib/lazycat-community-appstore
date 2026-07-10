package main

import (
	"os"

	"lazycat.community/appstore/internal/servercmd"
)

func main() {
	os.Exit(servercmd.Execute())
}
