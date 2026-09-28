/* dialects/c/testdata/broken.c — a syntax error the parser must survive
 * (definitions before and after the broken one still extract). */

int good(void) {
  return helper();
}

int broken(int a, @#$) {
  return a;
}

struct Survivor {
  int ok;
};
