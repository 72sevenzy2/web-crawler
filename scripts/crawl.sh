#!/bin/bash

# crawl test
cd .. || exit 1
go test ./tests/crawl_test.go -v
