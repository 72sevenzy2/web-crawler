#!/bin/bash

# invalid extract test
cd .. || exit 1
go test ./test/invalidExtract_test.go -v
