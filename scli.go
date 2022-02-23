package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type SCLIClient struct {
	path string
}

var scliClient *SCLIClient

const sclibridgePath = "/usr/local/bin/sclibridge"

func init() {
	if _, err := os.Stat(sclibridgePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("Requires access to sclibridge")
			os.Exit(1)
		}
	}
	scliClient = &SCLIClient{path: sclibridgePath}
}

func (c *SCLIClient) Run(option string, args ...string) ([]string, error) {
	cmdArgs := append([]string{option}, args...)
	cmd := exec.Command(sclibridgePath, cmdArgs...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.TrimLeft(string(b), "\n"), "\n"), nil
}
