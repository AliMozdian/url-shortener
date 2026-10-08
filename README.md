## Performance & Profiling (Part 3)

### Benchmark Results
Executed on an Intel(R) Core(TM) i7-13620H (16 threads) via `go test -bench=. -benchmem -benchtime=2s ./internal/api/`:

```text
BenchmarkHandleShorten-16     233398    8853 ns/op    7475 B/op    41 allocs/op
BenchmarkHandleRedirect-16    391255    5694 ns/op    6383 B/op    23 allocs/op
