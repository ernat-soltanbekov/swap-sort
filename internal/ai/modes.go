package ai

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ernat-soltanbekov/swap-sort/internal/operations"
	"github.com/ernat-soltanbekov/swap-sort/internal/sorter"
)

// Keeping full traces and prompts bounded is separate from sorting capacity.
const MaxCoachValues = 100
const MaxChatMessageBytes = 8192
const MaxChatTurns = 20

type Coach struct {
	Client *Client
	// Injection lets tests exercise faulty algorithms without adding a hidden
	// production mode that corrupts the real sorter's output.
	Sort func([]int64) sorter.Result
}

func New(config Config) *Coach { return &Coach{Client: NewClient(config), Sort: sorter.Sort} }
func ValidMode(mode string) bool {
	return mode == "explain" || mode == "debug" || mode == "compare" || mode == "chat"
}

// output remembers the first write failure so no successful exit is reported
// when stdout breaks halfway through a report.
type output struct {
	io.Writer
	err error
}

func (out *output) printf(format string, args ...any) {
	if out.err == nil {
		_, out.err = fmt.Fprintf(out.Writer, format, args...)
	}
}

func (c *Coach) Run(ctx context.Context, mode string, values []int64, input io.Reader, writer io.Writer) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !ValidMode(mode) {
		return errors.New("unknown coach mode")
	}
	if len(values) == 0 || len(values) > MaxCoachValues {
		return fmt.Errorf("coach requires 1 to %d values", MaxCoachValues)
	}
	out := &output{Writer: writer}
	result := c.Sort(values)
	out.printf("Operations generated (%d): %s\n", len(result.Operations), displayOperations(result.Operations))
	if out.err != nil {
		return out.err
	}
	trace, correct := executionTrace(values, result.Operations)
	switch mode {
	case "debug":
		out.printf("\n=== AI Coach Debug ===\n%s", trace)
		if correct {
			out.printf("Result: OK — A is sorted and B is empty.\n")
			return out.err
		}
		out.printf("Result: KO — the sequence did not sort the input.\n")
		c.answer(ctx, prompt(mode, values, result, trace), mockDebug(values), out)
	case "chat":
		return c.chat(ctx, values, result, input, out)
	case "compare":
		out.printf("\n=== AI Coach Comparison ===\n%s\n", measured(result))
		c.answer(ctx, prompt(mode, values, result, ""), mockCompare(values, result), out)
	default:
		out.printf("\n=== AI Coach Explanation ===\n")
		c.answer(ctx, prompt(mode, values, result, ""), mockExplain(values, result, trace), out)
	}
	return errors.Join(out.err, ctx.Err())
}

func displayOperations(ops []string) string {
	if len(ops) == 0 {
		return "(none)"
	}
	return strings.Join(ops, " ")
}

func executionTrace(values []int64, ops []string) (string, bool) {
	m := operations.New(values)
	var trace strings.Builder
	fmt.Fprintf(&trace, "Initial: A=%v B=%v\n", m.A.Values(), m.B.Values())
	for i, op := range ops {
		if err := m.Apply(op); err != nil {
			fmt.Fprintf(&trace, "Step %d: invalid operation %q; execution stopped.\n", i+1, op)
			return trace.String(), false
		}
		fmt.Fprintf(&trace, "%d. After %s: A=%v B=%v\n", i+1, op, m.A.Values(), m.B.Values())
	}
	fmt.Fprintf(&trace, "Final: A=%v B=%v\n", m.A.Values(), m.B.Values())
	return trace.String(), m.Sorted()
}

// Both network and mock replies pass through the same renderer and history.
func (c *Coach) answer(ctx context.Context, messages []Message, fallback string, out *output) string {
	answer := fallback
	if c.Client.Configured() {
		if real, err := c.Client.Complete(ctx, messages); err == nil {
			out.printf("Backend: configured model\n")
			answer = real
		} else {
			out.printf("Backend unavailable: %s. Using deterministic mock.\n", err)
			out.printf("Backend: mock (offline)\n")
		}
	} else {
		out.printf("Backend: mock (offline)\n")
	}
	out.printf("%s\n", answer)
	return answer
}

func mockExplain(values []int64, result sorter.Result, trace string) string {
	strategy := "Strategy: " + result.Strategy + ".\n"
	switch {
	case result.Strategy == "exact breadth-first search":
		strategy += "Explore possible stack states by distance from the sorted goal; follow the recorded shortest path.\n"
	case strings.HasPrefix(result.Strategy, "rank bands"):
		strategy += "Move values to B in rank bands, then return the largest remaining value first.\n"
	case result.Strategy == "greedy paired rotations":
		strategy += "Keep B in circular descending order, then merge into ascending A; rotate both stacks together when this saves moves.\n"
	case result.Strategy == "binary radix":
		strategy += "Group ranks by one binary digit at a time; repeat until every bit has been processed.\n"
	}
	if len(result.Operations) == 0 {
		return strategy + "Step by step: the input is already sorted; no changes are needed.\nEfficiency: 0 operations is optimal.\n" + improvements
	}
	note := "This is the shortest candidate measured here; a global minimum is not claimed."
	if len(values) <= 7 {
		note = "Breadth-first search proves this sequence has the minimum number of operations."
	}
	// Small examples show every step. Large examples show the start and end;
	// debug mode remains the place for the complete, unabridged trace.
	lines := strings.Split(strings.TrimSpace(trace), "\n")
	if len(lines) > 14 {
		lines = append(append(lines[:10:10], "... intermediate states omitted; use debug for every operation ..."), lines[len(lines)-3:]...)
	}
	return fmt.Sprintf("%sStep by step: pa/pb push; sa/sb/ss swap; ra/rb/rr rotate up; rra/rrb/rrr rotate down.\n%s\nEfficiency: %d operations for %d values. %s\n%s", strategy, strings.Join(lines, "\n"), len(result.Operations), len(values), note, improvements)
}

const improvements = "Code improvements: keep parsing, planning and HTTP transport separate; add a regression test for each failing input; verify operations with an independent interpreter; propagate cancellation and output errors when extending the CLI. These are maintenance recommendations, not claims of detected defects."

func mockDebug(values []int64) string {
	// Diagnose only observable invariants, then supply a verified replacement.
	fixed := sorter.Sort(values)
	return fmt.Sprintf("Analysis: the recorded sequence violates the required final state (ascending A and empty B), or contains an invalid instruction. Inspect the complete trace above; the first decreasing pair alone does not identify a faulty move.\nSuggested fix: replace the faulty sequence with this verified %d-operation plan: %s\n%s", len(fixed.Operations), displayOperations(fixed.Operations), improvements)
}
func measured(result sorter.Result) string {
	var text strings.Builder
	text.WriteString("Measured operation counts:\n")
	if len(result.Candidates) == 0 {
		text.WriteString("  already sorted: 0\n")
	}
	for _, candidate := range result.Candidates {
		fmt.Fprintf(&text, "  %s: %d\n", candidate.Strategy, candidate.Count)
	}
	return text.String()
}
func mockCompare(values []int64, result sorter.Result) string {
	return fmt.Sprintf("Strategy: compare exact search for small lists, binary radix, rank bands, and greedy paired rotations.\nEfficiency: for n=%d, radix has an upper estimate of 2*n*ceil(log2(n)) operations; bands and greedy plans depend on the permutation. Exact BFS uses (n+1)*n! states and is limited to n<=7. Counts above are measurements, not estimates.\nRecommendation: %s, with %d operations on this input. The default planner selects the shortest measured candidate.\n%s", len(values), result.Strategy, len(result.Operations), improvements)
}

func (c *Coach) chat(ctx context.Context, values []int64, result sorter.Result, input io.Reader, out *output) error {
	out.printf("\n=== AI Coach Chat ===\nAsk about the algorithm. Type exit to finish. Up to %d turns; all turns retain context.\n", MaxChatTurns)
	messages := prompt("chat", values, result, "")
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), MaxChatMessageBytes+1)
	for turn := 1; turn <= MaxChatTurns; {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		out.printf("You> ")
		if out.err != nil {
			return out.err
		}
		if !scanner.Scan() {
			if scanner.Err() != nil {
				return errors.New("chat input failed or exceeded 8192 bytes")
			}
			out.printf("\nSession ended.\n")
			return out.err
		}
		question := strings.TrimSpace(scanner.Text())
		if len(scanner.Bytes()) > MaxChatMessageBytes {
			return errors.New("chat input exceeded 8192 bytes")
		}
		if question == "exit" {
			out.printf("Session ended.\n")
			return out.err
		}
		if question == "" {
			continue
		}
		messages = append(messages, Message{"user", question})
		fallback := mockChat(question, turn, result, messages)
		answer := c.answer(ctx, messages, fallback, out)
		messages = append(messages, Message{"assistant", answer})
		if out.err != nil {
			return out.err
		}
		turn++
	}
	out.printf("Session limit reached (%d turns); start another chat to continue.\n", MaxChatTurns)
	return out.err
}

func mockChat(question string, turn int, result sorter.Result, messages []Message) string {
	text := strings.ToLower(question)
	var reply string
	switch {
	case strings.Contains(text, "previous") || strings.Contains(text, "предыдущ"):
		reply = "No earlier question in this session."
		for i := len(messages) - 2; i >= 2; i-- {
			if messages[i].Role == "user" {
				previous := []rune(terminalText(messages[i].Content))
				if len(previous) > 160 {
					previous = append(previous[:160], []rune("...")...)
				}
				reply = "Your previous question was: " + string(previous)
				break
			}
		}
	case strings.Contains(text, "test") || strings.Contains(text, "code") || strings.Contains(text, "код") || strings.Contains(text, "тест"):
		reply = improvements
	case strings.Contains(text, "pa") || strings.Contains(text, "pb"):
		reply = "Step by step: pb moves A's top to B; pa moves B's top back to A. An empty source makes a push a no-op. The final condition requires B to be empty."
	case strings.Contains(text, "rotat") || strings.Contains(text, "ra") || strings.Contains(text, "вращ"):
		reply = "Step by step: ra moves A's top to its bottom; rra moves its bottom to its top. rb and rrb do the same for B. rr and rrr rotate both stacks in one instruction."
	default:
		reply = fmt.Sprintf("Strategy: %s produced %d operations for the original input. The offline mock answers a fixed set of operation/testing questions; a configured model can answer open-ended questions. Ask about pa/pb, rotations or tests.", result.Strategy, len(result.Operations))
	}
	return fmt.Sprintf("Context: turn %d; %d earlier question(s) retained.\n%s", turn, turn-1, reply)
}
