// Package parser validates numbers and the line-oriented instruction language.
package parser

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// Limits keep accidental or hostile input from exhausting memory or CPU.
const MaxValues = 10000
const MaxArgumentBytes = 1 << 20
const MaxOperations = 2000000

var ErrInput = errors.New("invalid input")

// Numbers accepts quoted lists, separate arguments, or a mixture of both.
// Values are signed 64-bit integers; duplicates are compared numerically.
func Numbers(args []string) ([]int64, error) {
	values := make([]int64, 0)
	seen := make(map[int64]bool)
	bytes := 0
	for _, arg := range args {
		bytes += len(arg)
		if bytes > MaxArgumentBytes {
			return nil, ErrInput
		}
		fields := strings.Fields(arg)
		if len(fields) == 0 {
			return nil, ErrInput
		}
		for _, field := range fields {
			value, err := strconv.ParseInt(field, 10, 64)
			if err != nil || seen[value] || len(values) == MaxValues {
				return nil, ErrInput
			}
			seen[value] = true
			values = append(values, value)
		}
	}
	return values, nil
}

// Instructions streams complete lines to apply. It accepts CRLF and one extra
// trailing blank line because the audit's echo commands append that extra LF.
// Blank lines inside a sequence and unterminated instructions are errors.
func Instructions(input io.Reader, apply func(string) error) error {
	reader := bufio.NewReaderSize(input, 16)
	count, trailer := 0, false
	for {
		line, err := reader.ReadSlice('\n')
		if err == io.EOF && len(line) == 0 {
			return nil
		}
		if err != nil || len(line) > 5 {
			return ErrInput
		}
		name := strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
		if name == "" && count > 0 && !trailer {
			trailer = true
			continue
		}
		if trailer || name == "" || count == MaxOperations {
			return ErrInput
		}
		if err := apply(name); err != nil {
			return ErrInput
		}
		count++
	}
}
