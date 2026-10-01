.PHONY: all test audit stress race vet fmt-check fuzz clean

all:
	go build -trimpath -o push-swap ./cmd/push-swap
	go build -trimpath -o checker ./cmd/checker
	go build -trimpath -o ai-coach ./cmd/ai-coach

test:
	go test ./...

audit: all
	python3 scripts/audit.py

stress:
	SWAP_SORT_STRESS=10000 go test -count=1 -run 'TestRandomHundredBudget|TestSizesAndPathologicalOrders' -v ./internal/sorter

race:
	go test -race ./...

vet:
	go vet ./...

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)

fuzz:
	go test ./internal/parser -run '^$$' -fuzz FuzzNumbers -fuzztime=10s
	go test ./internal/parser -run '^$$' -fuzz FuzzInstructions -fuzztime=10s
	go test ./internal/sorter -run '^$$' -fuzz FuzzSort -fuzztime=10s
	go test ./internal/ai -run '^$$' -fuzz FuzzResponse -fuzztime=10s

clean:
	rm -f push-swap checker ai-coach coverage.out
