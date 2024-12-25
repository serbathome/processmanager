#!/bin/bash

rm -rf ./aefuf ./aefad ./pm

go build -o ./aefuf ./aefuf_src/aefuf.go
go build -o ./aefad ./aefad_src/aefad.go
go build -o ./pm main.go