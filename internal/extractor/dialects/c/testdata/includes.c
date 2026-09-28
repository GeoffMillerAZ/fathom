/* dialects/c/testdata/includes.c — imports.scm coverage: a quoted
 * (relative/include-path) include and a system angle-bracket include. */

#include "includes_local.h"
#include <stdio.h>

int use_local(void) {
  return local_value();
}
