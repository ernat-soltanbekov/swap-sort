package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ernat-soltanbekov/swap-sort/internal/ai"
)

type forbiddenReader struct{}

func (forbiddenReader) Read([]byte) (int, error) { panic("stdin must not be read") }

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken output") }

func TestExactCLIAudit(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := PushSwap(nil, &out, &stderr); code != 0 || out.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("no-argument sorter")
	}
	if code := Checker(nil, forbiddenReader{}, &out, &stderr); code != 0 || out.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("no-argument checker")
	}
	for _, arg := range []string{"0 one 2 3", "1 2 2 3", ""} {
		out.Reset()
		stderr.Reset()
		if code := PushSwap([]string{arg}, &out, &stderr); code != 1 || out.Len() != 0 || stderr.String() != "Error\n" {
			t.Fatal("wrong sorter error", out.String(), stderr.String())
		}
		out.Reset()
		stderr.Reset()
		if code := Checker([]string{arg}, forbiddenReader{}, &out, &stderr); code != 1 || out.Len() != 0 || stderr.String() != "Error\n" {
			t.Fatal("wrong checker error")
		}
	}
	for _, test := range []struct{ values, ops, want string }{
		{"0 9 1 8 2 7 3 6 4 5", "sa\npb\nrrr\n\n", "KO\n"},
		{"0 9 1 8 2", "pb\nra\npb\nra\nsa\nra\npa\npa\n\n", "OK\n"},
		{"3 2 1 0", "rra\npb\nsa\nrra\npa\n", "OK\n"},
		{"1 2 3", "", "OK\n"}, {"3 2 1", "", "KO\n"},
	} {
		out.Reset()
		stderr.Reset()
		if code := Checker([]string{test.values}, strings.NewReader(test.ops), &out, &stderr); code != 0 || out.String() != test.want || stderr.Len() != 0 {
			t.Fatal(test, out.String(), stderr.String())
		}
	}
	out.Reset()
	stderr.Reset()
	if code := PushSwap([]string{"0 1 2 3 4 5"}, &out, &stderr); code != 0 || out.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("sorted input")
	}
	out.Reset()
	stderr.Reset()
	if code := PushSwap([]string{"4 67 3 87 23"}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	instructions := out.String()
	out.Reset()
	if code := Checker([]string{"4 67 3 87 23"}, strings.NewReader(instructions), &out, &stderr); code != 0 || out.String() != "OK\n" {
		t.Fatal("pipeline failed")
	}
}

func TestMalformedCommandsAndOutputErrors(t *testing.T) {
	for _, text := range []string{"bad\n", "sa ", "sa\n\npb\n"} {
		var out, stderr bytes.Buffer
		code := Checker([]string{"2 1"}, strings.NewReader(text), &out, &stderr)
		if code != 1 || out.Len() != 0 || stderr.String() != "Error\n" {
			t.Fatal(text, code, out.String(), stderr.String())
		}
	}
	if PushSwap([]string{"2 1"}, brokenWriter{}, io.Discard) != 1 {
		t.Fatal("sorter write failure ignored")
	}
	if Checker([]string{"1"}, strings.NewReader(""), brokenWriter{}, io.Discard) != 1 {
		t.Fatal("checker write failure ignored")
	}
}

func TestCoachUsageAndModes(t *testing.T) {
	for _, args := range [][]string{nil, {"explain"}, {"debug", "1", "2"}, {"unknown", "1"}} {
		var out, stderr bytes.Buffer
		if code := AICoach(context.Background(), args, ai.Config{}, strings.NewReader(""), &out, &stderr); code != 1 || out.Len() != 0 || stderr.String() != Usage {
			t.Fatal(args, out.String(), stderr.String())
		}
	}
	for _, mode := range []string{"explain", "debug", "chat", "compare"} {
		var out, stderr bytes.Buffer
		if code := AICoach(context.Background(), []string{mode, "3 2 1"}, ai.Config{}, strings.NewReader("exit\n"), &out, &stderr); code != 0 || stderr.Len() != 0 || out.Len() == 0 {
			t.Fatal(mode, out.String(), stderr.String())
		}
	}
	var out, stderr bytes.Buffer
	if code := AICoach(context.Background(), []string{"explain", "1 1"}, ai.Config{}, strings.NewReader(""), &out, &stderr); code != 1 || out.Len() != 0 || stderr.String() != "Error\n" {
		t.Fatal("invalid coach input")
	}
}
