/* dialects/c/testdata/ifdefelse.c — an `else if` that opens a preprocessor
 * branch (#ifdef ... else if (...) { ... }) parses as a function definition
 * named `if` whose type is `else`; a statement keyword names no function
 * (LB-F117), so the calls inside belong to the function around it. */

int ready(int a);
int ask(int a);
int wait_for(int a);

static int check(int *ctx)
{
  if(ctx[0])
    return 1;
  else if(ctx[1])
    return 1;
#ifdef USE_ASYNC
  else if(ready(ctx[2])) {
    if(ask(ctx[3]))
      return 1;
    return wait_for(ctx[4]);
  }
#endif
  return 0;
}
