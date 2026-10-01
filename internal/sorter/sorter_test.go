package sorter

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/ernat-soltanbekov/swap-sort/internal/operations"
)

// This slice-based interpreter is intentionally independent of the production
// ring stack. Sharing the same executor in both tests and checker could hide a
// bug that affected both programs in the same way.
func reference(input []int64, ops []string) ([]int64, []int64) {
	a, b := make([]int64, len(input), len(input)), make([]int64, 0, len(input))
	copy(a, input)
	swap := func(s []int64) {
		if len(s) > 1 {
			s[0], s[1] = s[1], s[0]
		}
	}
	rotate := func(s []int64) {
		if len(s) > 1 {
			first := s[0]
			copy(s, s[1:])
			s[len(s)-1] = first
		}
	}
	reverse := func(s []int64) {
		if len(s) > 1 {
			last := s[len(s)-1]
			copy(s[1:], s)
			s[0] = last
		}
	}
	for _, op := range ops {
		switch op {
		case "pa":
			if len(b) > 0 {
				a = append(a, 0)
				copy(a[1:], a)
				a[0] = b[0]
				copy(b, b[1:])
				b = b[:len(b)-1]
			}
		case "pb":
			if len(a) > 0 {
				b = append(b, 0)
				copy(b[1:], b)
				b[0] = a[0]
				copy(a, a[1:])
				a = a[:len(a)-1]
			}
		case "sa":
			swap(a)
		case "sb":
			swap(b)
		case "ss":
			swap(a)
			swap(b)
		case "ra":
			rotate(a)
		case "rb":
			rotate(b)
		case "rr":
			rotate(a)
			rotate(b)
		case "rra":
			reverse(a)
		case "rrb":
			reverse(b)
		case "rrr":
			reverse(a)
			reverse(b)
		default:
			panic("unexpected instruction: " + op)
		}
	}
	return a, b
}

func verify(t *testing.T, values []int64, result Result) {
	t.Helper()
	a, b := reference(values, result.Operations)
	want := append([]int64(nil), values...)
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if fmt.Sprint(a) != fmt.Sprint(want) || len(b) != 0 {
		t.Fatalf("input=%v strategy=%s ops=%v final=%v/%v", values, result.Strategy, result.Operations, a, b)
	}
}
func permutations(values []int64, visit func([]int64)) {
	var walk func(int)
	walk = func(index int) {
		if index == len(values) {
			visit(values)
			return
		}
		for i := index; i < len(values); i++ {
			values[index], values[i] = values[i], values[index]
			walk(index + 1)
			values[index], values[i] = values[i], values[index]
		}
	}
	walk(0)
}

func TestEveryPermutationThroughSeven(t *testing.T) {
	for n := 1; n <= 7; n++ {
		values := make([]int64, n)
		for i := range values {
			values[i] = int64(i)*97 - 250
		}
		maxCount, count := 0, 0
		permutations(values, func(values []int64) {
			result := Sort(values)
			verify(t, values, result)
			maxCount = max(maxCount, len(result.Operations))
			count++
		})
		if n == 5 && maxCount >= 12 {
			t.Fatal("five-value audit budget exceeded", maxCount)
		}
		t.Logf("n=%d permutations=%d maximum operations=%d", n, count, maxCount)
	}
}

// Build distances with an independent slice interpreter and representation.
// BFS explores all stack splits, not only states with an empty B.
func TestExactDistancesIndependentOracle(t *testing.T) {
	for n := 2; n <= 5; n++ {
		goal := make([]int64, n)
		for i := range goal {
			goal[i] = int64(i)
		}
		type state struct {
			a, b     []int64
			distance int
		}
		keyOf := func(a, b []int64) string { return fmt.Sprint(a, "/", b) }
		queue := []state{{goal, nil, 0}}
		distances := map[string]int{keyOf(goal, nil): 0}
		for index := 0; index < len(queue); index++ {
			current := queue[index]
			for _, op := range operations.Names {
				// Reconstruct any A/B state using pushes, then apply the next move.
				joined := append(append([]int64(nil), current.b...), current.a...)
				setup := []string{}
				for range current.b {
					setup = append(setup, "pb")
				}
				// Initial B must be reversed before the setup pushes.
				for i, j := 0, len(current.b)-1; i < j; i, j = i+1, j-1 {
					joined[i], joined[j] = joined[j], joined[i]
				}
				a, b := reference(joined, append(setup, op))
				key := keyOf(a, b)
				if _, seen := distances[key]; seen {
					continue
				}
				distances[key] = current.distance + 1
				queue = append(queue, state{a, b, current.distance + 1})
			}
		}
		factorial := 1
		for i := 2; i <= n; i++ {
			factorial *= i
		}
		if len(distances) != (n+1)*factorial {
			t.Fatalf("missing oracle states n=%d: %d", n, len(distances))
		}
		permutations(goal, func(v []int64) {
			got := len(Sort(v).Operations)
			want := distances[keyOf(v, nil)]
			if got != want {
				t.Fatalf("%v distance %d != %d", v, got, want)
			}
		})
	}
}

func TestAuditSixAndIntegerEdges(t *testing.T) {
	for _, values := range [][]int64{{2, 1, 3, 6, 5, 8}, {4, 67, 3, 87, 23}, {math.MaxInt64, 0, math.MinInt64, -1, 1}, {9, 8, 7, 6, 5, 4, 3, 2, 1, 0}} {
		before := append([]int64(nil), values...)
		result := Sort(values)
		verify(t, values, result)
		if !reflect.DeepEqual(values, before) {
			t.Fatal("mutated caller input")
		}
		if len(values) == 6 && len(result.Operations) >= 9 {
			t.Fatalf("audit six-value case: %d", len(result.Operations))
		}
	}
}

func TestRandomHundredBudget(t *testing.T) {
	iterations := 300
	if raw := os.Getenv("SWAP_SORT_STRESS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			t.Fatal("invalid SWAP_SORT_STRESS")
		}
		iterations = value
	}
	random := rand.New(rand.NewSource(2008))
	maximum, total := 0, 0
	for trial := 0; trial < iterations; trial++ {
		values := make([]int64, 100)
		for i, value := range random.Perm(100) {
			values[i] = int64(value)*7919 - 400000
		}
		result := Sort(values)
		verify(t, values, result)
		count := len(result.Operations)
		if count >= 700 {
			t.Fatalf("trial=%d count=%d values=%v", trial, count, values)
		}
		maximum = max(maximum, count)
		total += count
	}
	t.Logf("seed=2008 cases=%d max=%d mean=%.2f limit=<700", iterations, maximum, float64(total)/float64(iterations))
}

func TestSizesAndPathologicalOrders(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	for _, n := range []int{8, 9, 10, 32, 64, 99, 100, 101, 250, 500, 512, 513, 1000, 10000} {
		for _, kind := range []string{"ascending", "descending", "rotated", "alternating", "random"} {
			values := make([]int64, n)
			for i := range values {
				switch kind {
				case "ascending":
					values[i] = int64(i)
				case "descending":
					values[i] = int64(n - 1 - i)
				case "rotated":
					values[i] = int64((i + n/2) % n)
				case "alternating":
					if i%2 == 0 {
						values[i] = int64(i / 2)
					} else {
						values[i] = int64(n - 1 - i/2)
					}
				}
			}
			if kind == "random" {
				for i, v := range random.Perm(n) {
					values[i] = int64(v)
				}
			}
			result := Sort(values)
			verify(t, values, result)
			if kind == "ascending" && len(result.Operations) != 0 {
				t.Fatal("sorted input emitted operations")
			}
			if n == 100 && len(result.Operations) >= 700 {
				t.Fatalf("%s needs %d", kind, len(result.Operations))
			}
		}
	}
}

func TestSimplificationPreservesArbitrarySequences(t *testing.T) {
	random := rand.New(rand.NewSource(13))
	for trial := 0; trial < 1000; trial++ {
		values := []int64{3, 1, 4, 2}
		ops := make([]string, 100)
		for i := range ops {
			ops[i] = operations.Names[random.Intn(len(operations.Names))]
		}
		a, b := reference(values, ops)
		x, y := reference(values, simplify(ops))
		if fmt.Sprint(a, b) != fmt.Sprint(x, y) {
			t.Fatal("optimizer changed final state", ops)
		}
	}
}

func TestDirectPlans(t *testing.T) {
	values := make([]int64, 100)
	for i := range values {
		values[i] = int64(i)
	}
	values[0], values[1] = values[1], values[0]
	result := Sort(values)
	verify(t, values, result)
	if len(result.Operations) != 1 || result.Operations[0] != "sa" {
		t.Fatal(result)
	}
	for i := range values {
		values[i] = int64((i + 75) % 100)
	}
	result = Sort(values)
	verify(t, values, result)
	if len(result.Operations) != 25 {
		t.Fatal(result)
	}
}

func TestConcurrentSort(t *testing.T) {
	var group sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			random := rand.New(rand.NewSource(int64(worker)))
			for n := 2; n <= 7; n++ {
				values := make([]int64, n)
				for i, value := range random.Perm(n) {
					values[i] = int64(value)
				}
				verify(t, values, Sort(values))
			}
		}(worker)
	}
	group.Wait()
}

func FuzzSort(f *testing.F) {
	f.Add([]byte{5, 4, 3, 2, 1})
	f.Add([]byte{0, 255, 10, 128, 1, 27, 64, 65, 66})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 256 {
			return
		}
		seen := map[byte]bool{}
		var values []int64
		for _, b := range data {
			if !seen[b] {
				seen[b] = true
				values = append(values, int64(b)-128)
			}
		}
		verify(t, values, Sort(values))
	})
}
