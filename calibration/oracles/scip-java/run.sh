#!/usr/bin/env bash
# run.sh — scip-java over a Java calibration subject (tier A for calls,
# def_mapping, typed_recall, import_resolution and file edges; language-breadth
# §1.3, ADR-048 amendment D2; LB-F70).
#
#   run.sh <jdk-home> <module-dir> <out.scip> [build args...]
#
# <module-dir> is the directory the build tool runs in AND the SCIP source
# root: document paths are relative to it, so it is also the subject root the
# measurement is given (guava: <clone>/guava, the module directory, whose
# files are src/com/google/...). With no build args scip-java runs its default
# (maven: `clean verify -DskipTests`; gradle: `clean build`); build args after
# the output replace it. A Maven module needs ./mvnw in <module-dir> (scip-java
# runs ./mvnw, else `mvn`; neither is installed): symlink the clone's wrapper
# (`ln -s ../mvnw mvnw; ln -s ../.mvn .mvn`) and `git clean` after. Offline
# proof: re-run with `-o` (maven) or `--offline` (gradle), 0 downloads.
#
# Pinned: com.sourcegraph:scip-java_2.13:0.12.3 (Apache-2.0; the newest release
# `coursier complete-dep com.sourcegraph:scip-java_2.13:` lists on 2026-10-01),
# launched with coursier under JDK 21.0.12.1+1.
set -euo pipefail
JDK=$1 mod=$2 out=$3; shift 3
export JAVA_HOME=$JDK PATH=$JDK/bin:$PATH
cd "$mod"
if [ $# -gt 0 ]; then trail=(-- "$@"); else trail=(); fi
nice -n 10 perl -e 'alarm 2400; exec @ARGV' coursier launch -q com.sourcegraph:scip-java_2.13:0.12.3 -- index --output "$out" "${trail[@]}"
