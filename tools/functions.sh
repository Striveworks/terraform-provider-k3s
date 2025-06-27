#!/bin/bash

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(realpath "${CURRENT_DIR}/..")"

function test_standup() {
    local test_dir="testdata/infra"

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
    local test_dir="testdata/infra"

    if ! cd "${ROOT_DIR}/${test_dir}"; then
        echo "Could not find '${test_dir}'"
        exit 0
    fi

    echo "Running destroy"
    tofu destroy -auto-approve
}

function testacc() {
    temp_file=$(mktemp)

    echo "Log dir $temp_file"

    TEST_JSON_PATH="${ROOT_DIR}/_vars.test.auto.tfvars.json" \
        TF_LOG_PROVIDER=TRACE \
        TF_LOG_PATH="${temp_file}" \
        TF_ACC=1 \
        go test \
        -run ^TestAcc \
        -parallel 5 \
        -v -cover \
        -timeout 30m ./... 2>&1 &
    testpid=$!

    tail -f "${temp_file}" | grep --line-buffered 'provider.terraform-provider-k3s' &
    logpid=$!
    wait -n

    if kill -0 "$testpid" 2>/dev/null; then
        kill "$testpid"
    fi
    if kill -0 "$logpid" 2>/dev/null; then
        kill "$logpid"
    fi
}

function tofu_wrapped() {
    temp_file=$(mktemp)

    # shellcheck disable=SC2068
    TF_LOG_PROVIDER=TRACE \
        TF_LOG_PATH="${temp_file}" \
        tofu $@ 2>&1 &

    cmdpid=$!
    tail -f "${temp_file}" | grep --line-buffered 'provider.terraform-provider-k3s' &
    logpid=$!

    wait -n

    if kill -0 "$cmdpid" 2>/dev/null; then
        kill "$cmdpid"
    fi
    if kill -0 "$logpid" 2>/dev/null; then
        kill "$logpid"
    fi

}
