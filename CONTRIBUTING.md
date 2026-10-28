# Contributing

A useful bug report includes the Go version, a small dataset, the operation,
and the expected and actual results. Remove private data from workbook samples.
Check existing issues and pull requests before starting overlapping work.

Keep the core dependency-free and the API focused. New behavior should include
behavioral tests covering nulls, errors, and input immutability. A performance
change needs a reproducible benchmark with allocations and its tradeoffs.

Run `go test ./...`, `go vet ./...`, and `go test -race ./...`. For browser changes,
run `bash scripts/build-demo.sh` and exercise the actual WebAssembly engine.

Use concise English commits and PR descriptions explaining the concrete problem,
resulting behavior, and validation. Tool-assisted contributions are welcome;
contributors remain responsible for understanding and verifying submitted code.
