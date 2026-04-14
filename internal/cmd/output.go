package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// PrintJSON writes JSON to stdout, respecting OutputFormat().
func PrintJSON(raw []byte) error {
	if len(raw) == 0 {
		fmt.Println("{}")
		return nil
	}
	if OutputFormat() == "json" {
		var buf bytes.Buffer
		if err := json.Compact(&buf, raw); err != nil {
			_, err = os.Stdout.Write(raw)
			return err
		}
		_, err := os.Stdout.Write(buf.Bytes())
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout)
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		_, err = os.Stdout.Write(raw)
		return err
	}
	_, err := os.Stdout.Write(buf.Bytes())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout)
	return err
}

// PrintRaw writes bytes as-is (e.g. log text).
func PrintRaw(b []byte) error {
	_, err := os.Stdout.Write(b)
	if err != nil {
		return err
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		_, err = fmt.Fprintln(os.Stdout)
	}
	return err
}
