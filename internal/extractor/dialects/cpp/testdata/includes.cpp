/* dialects/cpp/testdata/includes.cpp — imports.scm coverage: a quoted
 * (relative/include-path) include and a system angle-bracket include. */

#include "includes_local.hpp"
#include <vector>

int use_local() {
  return local_value();
}
