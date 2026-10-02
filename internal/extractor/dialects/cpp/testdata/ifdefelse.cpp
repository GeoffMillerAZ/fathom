/* dialects/cpp/testdata/ifdefelse.cpp — a statement keyword names no
 * function (LB-F117): an `else if` opening a preprocessor branch parses as a
 * definition named `if` (nlohmann's serializer shows the same for a `switch`
 * after a macro written as an access specifier); the calls inside belong to
 * the function around them. */

int ready(int a);
int ask(int a);
int wait_for(int a);

static int check(int *ctx)
{
  if (ctx[0])
    return 1;
  else if (ctx[1])
    return 1;
#ifdef USE_ASYNC
  else if (ready(ctx[2])) {
    if (ask(ctx[3]))
      return 1;
    return wait_for(ctx[4]);
  }
#endif
  return 0;
}
