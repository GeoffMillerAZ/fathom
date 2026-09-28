#!/usr/bin/env bash
# dialects/bash/testdata/multiline.sh — the brace-on-its-own-line function
# style (common in shell style guides): @body still anchors Signature to
# the definition's start, and EndLine still reads the closing brace's line.

function build_release()
{
    echo "building"
    echo "packaging"
}
