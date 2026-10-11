#!/usr/bin/env bash
source ./lib/helpers.sh
. "lib/config.sh" extra args
\. ../common.sh
source 'quoted path.sh'
builtin source lib/builtin.sh
source "$DIR/dynamic.sh"
. $(dirname "$0")/sub.sh
source lib/*.sh
source ~/home.sh
source "lib/$name.sh"
setup() {
  source "$HOME/x.sh"
  . ./inner.sh
}
source
echo source not.sh

\. "tail.sh"

X="lib/a.sh"

\. ./after-assign.sh
