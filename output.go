package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// emit writes either pretty JSON or a human-readable text summary, based on
// the chosen output format. textFn renders the text form.
func emit(format string, v any, textFn func()) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case "text":
		textFn()
		return nil
	default:
		return fmt.Errorf("unknown output format %q (use json or text)", format)
	}
}
