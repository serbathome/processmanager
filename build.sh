#!/bin/bash

rm -rf ./aefuf ./aefad ./pm

go build -o ./aefuf ./aefuf_mock/aefuf.go
go build -o ./aefad ./aefad_mock/aefad.go
go build -o ./pm main.go