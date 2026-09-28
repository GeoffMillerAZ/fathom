/* dialects/c/testdata/shapes.c — coverage fixture: every required kind, a
 * multi-line signature, plain/pointer/arrow calls, and a macro pair. */

#define MAX_SIZE 128

#define SQUARE(x) ((x) * (x))

enum Color {
  COLOR_RED,
  COLOR_GREEN,
  COLOR_BLUE
};

struct Point {
  int x;
  int y;
};

union Value {
  int as_int;
  float as_float;
};

typedef struct Point PointAlias;

typedef int (*Callback)(int);

struct Handler {
  Callback run;
};

static int helper(int a) {
  return SQUARE(a);
}

int add(
    int a,
    int b
) {
  return helper(a) + b;
}

void *make_point(int x, int y) {
  struct Point *p;
  p = 0;
  return p;
}

int dispatch(struct Handler *ctx, int v) {
  return ctx->run(v);
}
