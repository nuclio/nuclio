#!/bin/bash

. support/scripts/functions.sh

checkfmt() {
    local files="$(gofmt -l $(local_go_pkgs))"
    if [ -n "$files" ]; then
        echo "You need to run \"gofmt -w ./\" to fix your formating."
        echo "$files" >&2
        return 1
    fi
}

lint() {
    gometalinter \
        --exclude=_mock.go \
        --disable=gotype  \
        --disable=golint \
        --vendor \
        --skip=test \
        --fast \
        --deadline=600s \
        --severity=golint:error \
        --errors \
        $(local_go_pkgs)
}

scanast() {
    set +e
    gosec ./... > security.log 2>&1
    set -e

    local issues=$(grep -E "Severity: MEDIUM" security.log | wc -l)
    if [ -n $issues ] && [ $issues -gt 0 ]; then
        echo ""
        echo "Medium Severity Issues:"
        grep -E "Severity: MEDIUM" -A 1 security.log
        echo $issues "medium severity issues found."
    fi

    local issues=$(grep -E "Severity: HIGH" security.log | grep -v "vendor")
    local issues_count=$(grep -E "Severity: HIGH" security.log | grep -v "vendor" | wc -l)
    if [ -n $issues_count ] && [ $issues_count -gt 0 ]; then
        echo ""
        echo "High Severity Issues:"
        grep -E "Severity: HIGH" -A 1 security.log
        echo $issues_count "high severity issues found."
        echo $issues
        echo "You need to resolve the high severity issues at the least."
        exit 1
    fi

    local issues=$(grep -E "Errors unhandled" security.log | grep -v "vendor" | grep -v "/src/go/src")
    local issues_count=$(grep -E "Errors unhandled" security.log | grep -v "vendor" | grep -v "/src/go/src" | wc -l)
    if [ -n $issues_count ] && [ $issues_count -gt 0 ]; then
        echo ""
        echo "Unhandled errors:"
        grep -E "Errors unhandled" security.log
        echo $issues_count "unhandled errors, please indicate with the right comment that this case is ok, or handle the error."
        echo $issues
        echo "You need to resolve the all unhandled errors."
        exit 1
    fi
    rm security.log
}

usage() {
    echo "check.sh fmt|lint" >&2
    exit 2
}

case "$1" in
    fmt) checkfmt ;;
    lint) lint ;;
    scanast) scanast;;
    *) usage ;;
esac
