#include "outer.h"

int run(void) {
	api_open(1);
	api_twice(2);
	api_missing(3);
	api_inline(4);
	api_dual(5);
	api_wrapped(6);
	stray(7);
	return 0;
}
