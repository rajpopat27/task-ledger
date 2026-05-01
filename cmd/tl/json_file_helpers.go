package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func outputJSON(v interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func ensureStoreActive() error {
	storeMutex.Lock()
	active := storeActive && store != nil
	storeMutex.Unlock()
	if active {
		return nil
	}
	return initializeFileBackendForCommand()
}
