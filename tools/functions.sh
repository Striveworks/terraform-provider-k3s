#!/bin/bash

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(realpath "${CURRENT_DIR}/..")"

function test_standup() {
    local test_dir="tests"

    if ! cd "${ROOT_DIR}/${test_dir}"; then
        echo "Could not find '${test_dir}'"
        exit 0
    fi

    echo "Running init"
    tofu init

    echo "Running apply"
    tofu apply -auto-approve

    echo "Extracting outputs"
    tofu output -json | jq 'to_entries | map({(.key): .value.value})|add' >"${ROOT_DIR}/_vars.test.auto.tfvars.json"
}

function test_teardown() {
    local test_dir="tests"

    if ! cd "${ROOT_DIR}/${test_dir}"; then
        echo "Could not find '${test_dir}'"
        exit 0
    fi

    echo "Running destroy"
    tofu destroy -auto-approve
}

function testacc() {

    TEST_JSON_PATH="${ROOT_DIR}/_vars.test.auto.tfvars.json" \
        TF_LOG_PROVIDER=TRACE \
        TF_LOG=TRACE \
        TF_LOG_PATH="${ROOT_DIR}/acc-test.log" \
        TF_ACC=1 \
        go test \
        -run ^TestAcc \
        -parallel 5 \
        -v -cover \
        -timeout 30m ./... >"${ROOT_DIR}/acc-test.results.log" 2>&1 &

    testpid=$!

    if [ -f "${ROOT_DIR}/acc-test.log" ]; then
        tail -f "${ROOT_DIR}/acc-test.log" &
        logpid=$!
    fi

    wait -n

    kill "$logpid" "$testpid"

    cat "${ROOT_DIR}/acc-test.results.log"

}
