#!/bin/bash
go test ./... -coverprofile=c.out
go tool cover -html=c.out -o coverage.html