package main

import (
	"os"

	"lazycat.community/appstore/internal/clientcmd"
)

func main() {
	os.Exit(clientcmd.Execute())
}
