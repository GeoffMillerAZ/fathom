#include "../inc/types.h"

/* this file defines item_t itself (a build that selects its own); the
   header's item_t is not the one its uses mean */
typedef struct own_s { int z; } item_t;

int shadow_total(item_t *o) {
  return o->z;
}
