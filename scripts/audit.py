#!/usr/bin/env python3
"""Black-box audit of compiled programs; no Python packages or model required."""

import itertools
import json
import os
from pathlib import Path
import random
import subprocess
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = Path(__file__).resolve().parent.parent
ENV = {key: value for key, value in os.environ.items() if not key.startswith("LLM_")}
CHECKS = 0


def run(program, *args, stdin="", config=None, timeout=20):
    return subprocess.run(
        [str(ROOT / program), *args], input=stdin, text=True,
        capture_output=True, timeout=timeout, env={**ENV, **(config or {})},
    )


def expect(result, stdout=None, stderr="", code=0):
    global CHECKS
    assert result.returncode == code, (result.args, result.returncode, result.stderr)
    assert stdout is None or result.stdout == stdout, (result.args, result.stdout, stdout)
    assert result.stderr == stderr, (result.args, result.stderr, stderr)
    CHECKS += 1


def execute(values, commands):
    """Independent reference, using Python lists rather than Go ring stacks."""
    a, b = list(values), []
    for name in commands.splitlines():
        assert name in {"pa", "pb", "sa", "sb", "ss", "ra", "rb", "rr", "rra", "rrb", "rrr"}
        if name == "pa" and b:
            a.insert(0, b.pop(0))
        elif name == "pb" and a:
            b.insert(0, a.pop(0))
        for stack, suffix in ((a, "a"), (b, "b")):
            if name in ("s" + suffix, "ss") and len(stack) > 1:
                stack[0], stack[1] = stack[1], stack[0]
            if name in ("r" + suffix, "rr") and stack:
                stack.append(stack.pop(0))
            if name in ("rr" + suffix, "rrr") and stack:
                stack.insert(0, stack.pop())
    assert a == sorted(values) and not b, (values, commands, a, b)


def sort_case(values, budget=None):
    text = " ".join(map(str, values))
    result = run("push-swap", text)
    expect(result)
    assert not result.stdout or result.stdout.endswith("\n")
    count = len(result.stdout.splitlines())
    assert budget is None or count < budget, (values, count, budget)
    execute(values, result.stdout)
    expect(run("checker", text, stdin=result.stdout), "OK\n")
    return count


class Backend(BaseHTTPRequestHandler):
    records = []

    def log_message(self, *_):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        type(self).records.append((self.path, self.headers.get("Authorization"), body))
        if self.path.startswith("/slow/"):
            time.sleep(10.5)
        if self.path.startswith("/error/"):
            self.send_response(503)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        payload = {"choices": [{"message": {"role": "assistant", "content":
                   "Strategy: fixture reply.\nStep by step: verified request.\n"
                   "Efficiency: use measured counts.\nCode improvements: add regression tests."}}]}
        try:
            self.wfile.write(json.dumps(payload).encode())
        except (BrokenPipeError, ConnectionResetError):
            pass  # Expected after the client's ten-second deadline.


def audit_cli():
    expect(run("push-swap"), "")
    expect(run("checker", stdin="invalid\n"), "")
    expect(run("push-swap", "0 1 2 3 4 5"), "")
    expect(run("checker", "1 2 3"), "OK\n")
    for program in ("push-swap", "checker"):
        for bad in ("0 one 2 3", "1 2 2 3", "", " ", "0 -0", "01 +1", "2.5", "1e2",
                    "9223372036854775808", "-9223372036854775809", "１２ 1"):
            expect(run(program, bad), "", "Error\n", 1)
    for bad in ("bad\n", "sa", "sa \n", "sa\n\npb\n", "sa\n\n\n", "x" * 100000):
        expect(run("checker", "2 1", stdin=bad), "", "Error\n", 1)
    expect(run("checker", "2 1", stdin="sa\r\n"), "OK\n")
    expect(run("checker", "0 9 1 8 2 7 3 6 4 5", stdin="sa\npb\nrrr\n\n"), "KO\n")
    expect(run("checker", "0 9 1 8 2", stdin="pb\nra\npb\nra\nsa\nra\npa\npa\n\n"), "OK\n")
    expect(run("checker", "3 2 1 0", stdin="rra\npb\nsa\nrra\npa\n"), "OK\n")
    six = sort_case([2, 1, 3, 6, 5, 8], 9)
    five_max = max(sort_case(values, 12) for values in itertools.permutations([-51, -8, 0, 24, 999]))
    sort_case([4, 67, 3, 87, 23])
    sort_case([2**63 - 1, 0, -(2**63), -1, 1])
    split = run("push-swap", "3", "1 2")
    expect(split)
    execute([3, 1, 2], split.stdout)
    rng = random.Random(2008)
    maximum = max(sort_case(rng.sample(range(-1000000, 1000000), 100), 700) for _ in range(100))
    for name in ("test_small.txt", "test_medium.txt", "test_large.txt"):
        values = list(map(int, (ROOT / "testdata" / name).read_text().split()))
        sort_case(values, 700 if len(values) == 100 else None)
    print(f"CLI: six-value example={six}; all 120 five-value permutations max={five_max}; 100 random hundreds max={maximum}")


def audit_coach():
    usage = 'Usage: ./ai-coach <mode> "<input>"\nModes: explain, debug\n'
    for args in ([], ["explain"], ["debug", "1", "2"], ["unknown", "1"]):
        expect(run("ai-coach", *args), "", usage, 1)
    expect(run("ai-coach", "explain", "2 2"), "", "Error\n", 1)
    for mode in ("explain", "debug", "compare", "chat"):
        result = run("ai-coach", mode, "5 4 3 2 1", stdin="What is pb?\nWhat about tests?\nexit\n")
        expect(result)
        assert result.stdout == run("ai-coach", mode, "5 4 3 2 1", stdin="What is pb?\nWhat about tests?\nexit\n").stdout
        if mode == "debug":
            assert "Initial:" in result.stdout and "After " in result.stdout and "Result: OK" in result.stdout
        else:
            assert "Backend: mock (offline)" in result.stdout and "Code improvements:" in result.stdout
    result = run("ai-coach", "explain", "1 2 3 4 5")
    expect(result)
    assert "already sorted" in result.stdout and "0 operations is optimal" in result.stdout
    result = run("ai-coach", "chat", "2 1", stdin="What is pb?\nWhat was my previous question?\nexit\n")
    expect(result)
    assert "Your previous question was: What is pb?" in result.stdout

    server = ThreadingHTTPServer(("127.0.0.1", 0), Backend)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    base = f"http://127.0.0.1:{server.server_port}"
    try:
        for prefix, model, key in (("/v1", "model-one", ""), ("/compatible", "model-two", "fixture-key")):
            config = {"LLM_BASE_URL": base + prefix, "LLM_MODEL": model, "LLM_API_KEY": key}
            result = run("ai-coach", "explain", "5 4 3 2 1", config=config)
            expect(result)
            assert "Backend: configured model" in result.stdout
            path, auth, body = Backend.records[-1]
            assert path == prefix + "/chat/completions"
            assert auth == ("Bearer " + key if key else None)
            assert body["model"] == model
            assert "Input:" in body["messages"][1]["content"]
            assert "Operations" in body["messages"][1]["content"]
        config = {"LLM_BASE_URL": base + "/v1"}
        result = run("ai-coach", "chat", "3 2 1", stdin="first question\nsecond question\nexit\n", config=config)
        expect(result)
        messages = Backend.records[-1][2]["messages"]
        assert [m["role"] for m in messages] == ["system", "user", "user", "assistant", "user"]
        assert messages[2]["content"] == "first question" and messages[4]["content"] == "second question"
        for suffix in ("/error", "/slow"):
            start = time.monotonic()
            result = run("ai-coach", "explain", "3 2 1", config={"LLM_BASE_URL": base + suffix})
            elapsed = time.monotonic() - start
            expect(result)
            assert "Backend unavailable:" in result.stdout and "Backend: mock (offline)" in result.stdout
            if suffix == "/slow":
                assert 9 <= elapsed < 15, elapsed
                print(f"HTTP: actual timeout and offline fallback verified in {elapsed:.2f}s")
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)
    result = run("ai-coach", "compare", "3 2 1", config={"LLM_BASE_URL": base})
    expect(result)
    assert "Backend unavailable:" in result.stdout and "Recommendation:" in result.stdout


if __name__ == "__main__":
    audit_cli()
    audit_coach()
    print(f"PASS: {CHECKS} black-box process checks; all mandatory CLI cases and coach bonuses covered.")
