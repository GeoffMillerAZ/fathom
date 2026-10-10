#!/usr/bin/env bash
# install-log-deps.sh DIR [LANG...] — TL-4's logging libraries for the fixture
# apps' log capture runners (docs/plans/telemetry-links.md), installed into
# DIR/<lang>, never globally: structlog (python), pino and winston (node),
# Monolog (php), and SLF4J with log4j2 (java, jars from Maven Central,
# checked against their published SHA-1). Pinned versions; re-runnable.
# `indexbench localize-replay -logs -deps DIR` reads them from there.
set -euo pipefail

dir="${1:?usage: install-log-deps.sh DIR [python node php java]}"
shift
langs=("$@")
[ ${#langs[@]} -eq 0 ] && langs=(python node php java)
mkdir -p "$dir"
dir="$(cd "$dir" && pwd)"

STRUCTLOG=26.1.0
PINO=10.4.0
WINSTON=3.19.0
MONOLOG=3.12.1
SLF4J=2.0.17
LOG4J=2.25.2

for lang in "${langs[@]}"; do
  case "$lang" in
  python)
    python3 -m pip install --quiet --disable-pip-version-check --no-deps --target "$dir/python" "structlog==$STRUCTLOG"
    ;;
  node)
    mkdir -p "$dir/node"
    npm install --silent --no-audit --no-fund --prefix "$dir/node" "pino@$PINO" "winston@$WINSTON"
    ;;
  php)
    mkdir -p "$dir/php"
    composer require --quiet --no-interaction --working-dir "$dir/php" "monolog/monolog:$MONOLOG"
    ;;
  java)
    mkdir -p "$dir/java"
    for coord in "org/slf4j/slf4j-api/$SLF4J/slf4j-api-$SLF4J.jar" \
      "org/apache/logging/log4j/log4j-api/$LOG4J/log4j-api-$LOG4J.jar" \
      "org/apache/logging/log4j/log4j-core/$LOG4J/log4j-core-$LOG4J.jar" \
      "org/apache/logging/log4j/log4j-slf4j2-impl/$LOG4J/log4j-slf4j2-impl-$LOG4J.jar"; do
      jar="$dir/java/${coord##*/}"
      if [ ! -f "$jar" ]; then
        curl -fsSL -o "$jar.part" "https://repo1.maven.org/maven2/$coord"
        want="$(curl -fsSL "https://repo1.maven.org/maven2/$coord.sha1" | cut -c1-40)"
        got="$(shasum -a 1 "$jar.part" | cut -c1-40)"
        if [ "$want" != "$got" ]; then
          echo "install-log-deps: $coord: sha1 $got, want $want" >&2
          rm -f "$jar.part"
          exit 1
        fi
        mv "$jar.part" "$jar"
      fi
    done
    ;;
  *)
    echo "install-log-deps: unknown language $lang" >&2
    exit 2
    ;;
  esac
done
