package parser

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/ernat-soltanbekov/swap-sort/internal/operations"
)

func TestNumbers(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "[]"}, {[]string{"3 -2 +1"}, "[3 -2 1]"}, {[]string{"3", "-2 +1"}, "[3 -2 1]"},
		{[]string{"\t-9223372036854775808\n9223372036854775807"}, "[-9223372036854775808 9223372036854775807]"},
		{[]string{"001 -0"}, "[1 0]"},
	}
	for _, test := range cases {
		got, err := Numbers(test.args)
		if err != nil || fmt.Sprint(got) != test.want {
			t.Fatal(test.args, got, err)
		}
	}
	for _, bad := range []string{"", " ", "1 1", "0 -0", "01 +1", "1.0", "1e3", "0x10", "1_000", "--1", "+", "one", "１２", "1\x002", "9223372036854775808", "-9223372036854775809"} {
		if _, err := Numbers([]string{bad}); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, err := Numbers([]string{"1", ""}); err == nil {
		t.Fatal("empty argument accepted")
	}
	if _, err := Numbers([]string{strings.Repeat(" ", MaxArgumentBytes+1)}); err == nil {
		t.Fatal("oversized argument accepted")
	}
	args := make([]string, MaxValues+1)
	for i := range args {
		args[i] = fmt.Sprint(i)
	}
	if _, err := Numbers(args[:MaxValues]); err != nil {
		t.Fatal(err)
	}
	if _, err := Numbers(args); err == nil {
		t.Fatal("count limit not enforced")
	}
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func TestInstructions(t *testing.T) {
	for _, valid := range []string{"", "sa\n", "sa\r\n", "sa\npb\nrrr\n\n"} {
		m := operations.New([]int64{3, 2, 1})
		if err := Instructions(strings.NewReader(valid), m.Apply); err != nil {
			t.Fatalf("rejected %q", valid)
		}
	}
	for _, invalid := range []string{"\n", "sa", "sa \n", " sa\n", "sa\r\r\n", "sA\n", "ra\n\npb\n", "sa\n\n\n", "ss\x00\n", strings.Repeat("x", 1<<20)} {
		m := operations.New([]int64{3, 2, 1})
		if err := Instructions(strings.NewReader(invalid), m.Apply); err == nil {
			t.Fatalf("accepted %q", invalid[:min(len(invalid), 32)])
		}
	}
	if err := Instructions(failedReader{}, func(string) error { return nil }); err == nil {
		t.Fatal("read failure ignored")
	}
	if err := Instructions(io.LimitReader(strings.NewReader("sa\n"), 2), func(string) error { return nil }); err == nil {
		t.Fatal("truncated operation accepted")
	}
	if err := Instructions(strings.NewReader(strings.Repeat("sa\n", MaxOperations+1)), func(string) error { return nil }); err == nil {
		t.Fatal("operation budget not enforced")
	}
}
func FuzzNumbers(f *testing.F) {
	for _, seed := range []string{"", "1 2 -3", "0 -0", "9223372036854775808"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		values, err := Numbers([]string{text})
		if err != nil {
			return
		}
		seen := map[int64]bool{}
		for _, value := range values {
			if seen[value] {
				t.Fatal("duplicate accepted")
			}
			seen[value] = true
		}
		parts := make([]string, len(values))
		for i, v := range values {
			parts[i] = fmt.Sprint(v)
		}
		again, err := Numbers(parts)
		if err != nil || fmt.Sprint(again) != fmt.Sprint(values) {
			t.Fatal("round trip failed")
		}
	})
}
func FuzzInstructions(f *testing.F) {
	for _, seed := range []string{"", "sa\n", "pb\nra\npa\n", "\x00\n"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		m := operations.New([]int64{3, 1, 2})
		_ = Instructions(strings.NewReader(text), m.Apply)
		values := append(m.A.Values(), m.B.Values()...)
		seen := map[int64]bool{}
		for _, v := range values {
			if seen[v] || v < 1 || v > 3 {
				t.Fatal("value corrupted")
			}
			seen[v] = true
		}
		if len(values) != 3 {
			t.Fatal("value lost")
		}
	})
}
