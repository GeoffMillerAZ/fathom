#!/bin/bash
cd -- "$(dirname -- "$0")" || exit
. ./leaf.sh
. ./empty.sh
foo
. ./missing.sh 2>/dev/null || :
load() { . ./leaf.sh; }
load
cd one || exit
. ./child.sh
. ../defs.sh
defined_in_importer() { :; }
false
[ "$?" = 1 ] || exit 19
cd .. || exit
PWD="$PWD/fake"
. ./leaf.sh
