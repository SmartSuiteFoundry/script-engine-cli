package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// ReadJSONPayload loads JSON from --file path, or stdin if file is "-" or if stdin is piped.
func ReadJSONPayload(file string, emptyDefault []byte) ([]byte, error) {
	switch {
	case file != "" && file != "-":
		return os.ReadFile(file)
	case file == "-":
		return io.ReadAll(os.Stdin)
	default:
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(string(data)) == "" {
				if emptyDefault != nil {
					return emptyDefault, nil
				}
				return nil, fmt.Errorf("empty stdin; provide JSON body")
			}
			return data, nil
		}
		if emptyDefault != nil {
			return emptyDefault, nil
		}
		return nil, fmt.Errorf("provide --file path.json or pipe JSON to stdin")
	}
}
