#!/bin/bash

cd .. || exit 1
go test ./tests/requestRetry_test.go -v
