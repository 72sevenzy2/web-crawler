#!/bin/bash

cd .. || exit 1
cd benchmarks || exit 1

go test -bench BenchmarkCrawling -count 10 -memprofile=mem.prof > results.txt
benchstat results.txt

# open memory profiling afterwards
go tool pprof mem.prof
