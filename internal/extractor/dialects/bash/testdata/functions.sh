#!/usr/bin/env bash
# dialects/bash/testdata/functions.sh — both function-definition syntaxes,
# an exported var at top level and one inside a function body (both count),
# a local var and a plain top-level assignment (neither counts), and plain
# calls (a builtin, a call through a positional arg, and a call to another
# function defined in this file).

export API_KEY="abc123"

LOCAL_ONLY=1

function greet() {
    local name=$1
    echo "Hello, $name"
}

deploy() {
    export DEPLOY_STAGE="prod"
    greet "world"
    echo "deploying"
}
