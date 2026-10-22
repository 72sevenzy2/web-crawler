#!/bin/bash

cd .. || exit 1
cd benchmarks || exit 1

go test -bench BenchmarkCrawling -benchmem -count=20 -memprofile=mem.prof -cpuprofile=cpu.prof > results.txt
benchstat results.txt

# open memory profiling afterwards
go tool pprof mem.prof
