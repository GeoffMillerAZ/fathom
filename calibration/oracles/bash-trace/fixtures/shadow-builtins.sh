#!/usr/bin/env bash
cd -- "$(dirname -- "$0")" || exit
printf() { return 91; }
shopt() { return 92; }
declare() { return 93; }
compgen() { return 94; }
pwd() { return 95; }
. ./leaf.sh
. ./defs.sh
:
