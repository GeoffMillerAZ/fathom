#ifndef API_H
#define API_H

/* Declared here, defined in a .c file the includer never names. */
int api_open(int fd);
int api_close(int fd);
int api_twice(int v);          /* two .c files define it: ambiguous */
int api_missing(int v);        /* nobody defines it */
int api_dual(int v);           /* one static definition, one external */
extern "C" int api_wrapped(int v);
void (*api_hook)(int);         /* a function-pointer variable, not a prototype */
typedef int (*api_cb)(int);    /* a typedef, not a prototype */
struct api_ops {
	int (*run)(int);           /* a member, not a prototype */
};
static inline int api_inline(int v) { return v; }

#endif
