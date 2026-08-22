#!/bin/bash

# valid extract test
cd .. || exit 1
go test ./tests/extract_test.go -v
