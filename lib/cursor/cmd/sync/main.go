package main

import (
	"os"

	"cursor-sync/internal/syncer"
)

func main() {
	os.Exit(syncer.SyncRules())
}
