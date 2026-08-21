#!/bin/bash

# crawl test
cd .. || exit 1
go test ./test/crawl_test.go
