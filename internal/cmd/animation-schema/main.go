package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/roshbhatia/go-utils/animation"
)

func main() {
	check := flag.Bool("check", false, "fail when the generated schema differs")
	output := flag.String("output", "schema/terminal.animation.v1.schema.json", "schema output path")
	flag.Parse()

	generated, err := animation.Schema()
	if err != nil {
		fail(err)
	}
	if *check {
		current, err := os.ReadFile(*output)
		if err != nil {
			fail(err)
		}
		if !bytes.Equal(current, generated) {
			fail(fmt.Errorf("%s is stale; run go run ./internal/cmd/animation-schema", *output))
		}
		return
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*output, generated, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
