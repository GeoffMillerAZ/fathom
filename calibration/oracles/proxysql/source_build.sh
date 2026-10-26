#!/bin/sh
# Inside the pinned upstream packaging container; never on the host.
set -eu
cd /probe/source40
export GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=safe.directory GIT_CONFIG_VALUE_0=/probe/source40
[ "$(git rev-parse HEAD)" = 7ddb3dc015d1440e8a0fb0bbafa2df9aaaa2669d ]
[ -z "$(git status --porcelain)" ]
if ! pkg-config --exists protobuf; then
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends libprotobuf-dev
fi
# Bind mounts cannot preserve arbitrary archive owners. Source files stay unmodified.
export TAR_OPTIONS=--no-same-owner
exec nice -n 10 make PROXYSQL40=1 debug -j2
