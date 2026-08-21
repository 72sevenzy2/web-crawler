#!/bin/bash

# invalid extract test
cd .. || exit 1
go test ./tests/invalidExtract_test.go -v
