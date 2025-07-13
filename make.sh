#!/bin/bash

function print_help {
    echo "Commands:"
    HERE="$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )/make.sh"
    rg '^\s+\w+-*\w*-*\w*\)' "$HERE"
}

function check() {  # Run code checks: go fmt, go build, go test
    set -x
    if [ -n "$(gofmt -s -l .)" ]; then
        echo "gofmt required - files need formatting:"
        gofmt -s -l .
        return 1
    fi
    
    go build ./...
    go test ./...
}

function fix() {  # Auto-fix and format Go code
    set -x
    gofmt -s -w .
}

if [ -z "$1" ]; then print_help; exit 0; fi

CMD="$1"
shift
case "$CMD" in
    check)  # Run code checks: go fmt, go build, go test
        check ;;
    fix)  # Auto-fix and format Go code
        fix ;;
    help)  # Show help
        print_help ;;
    *)
        print_help ;;
esac
