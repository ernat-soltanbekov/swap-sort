// Package cli keeps process-independent entry points easy to exercise in tests.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/ernat-soltanbekov/swap-sort/internal/ai"
	"github.com/ernat-soltanbekov/swap-sort/internal/operations"
	"github.com/ernat-soltanbekov/swap-sort/internal/parser"
	"github.com/ernat-soltanbekov/swap-sort/internal/sorter"
)

func failure(stderr io.Writer) int { fmt.Fprintln(stderr, "Error"); return 1 }

func PushSwap(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return 0
	}
	values, err := parser.Numbers(args)
	if err != nil {
		return failure(stderr)
	}
	result := sorter.Sort(values)
	writer := bufio.NewWriter(stdout)
	for _, op := range result.Operations {
		if _, err := fmt.Fprintln(writer, op); err != nil {
			return failure(stderr)
		}
	}
	if err := writer.Flush(); err != nil {
		return failure(stderr)
	}
	return 0
}

func Checker(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return 0
	}
	values, err := parser.Numbers(args)
	if err != nil {
		return failure(stderr)
	}
	machine := operations.New(values)
	if err := parser.Instructions(stdin, machine.Apply); err != nil {
		return failure(stderr)
	}
	verdict := "KO"
	if machine.Sorted() {
		verdict = "OK"
	}
	if _, err := fmt.Fprintln(stdout, verdict); err != nil {
		return failure(stderr)
	}
	return 0
}

const Usage = "Usage: ./ai-coach <mode> \"<input>\"\nModes: explain, debug\n"

func AICoach(ctx context.Context, args []string, config ai.Config, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 2 || !ai.ValidMode(args[0]) {
		fmt.Fprint(stderr, Usage)
		return 1
	}
	values, err := parser.Numbers(args[1:])
	if err != nil {
		return failure(stderr)
	}
	if err := ai.New(config).Run(ctx, args[0], values, stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err)
		return 1
	}
	return 0
}
