/* dialects/cpp/testdata/calls.cpp — call shapes: plain, member through . and
 * ->, qualified, template, a cast (not a call), a lambda body and a call of a
 * local callable. */

#include <cstdio>

namespace util {
int twice(int v) { return v * 2; }
template <typename T> T ident(T v) { return v; }
}

struct Counter {
  int n = 0;
  void bump() { n = util::twice(n); }
  int read() const { return n; }
};

int helper(int a) { return a + 1; }

int drive(Counter &c, Counter *p) {
  c.bump();
  p->bump();
  int a = helper(c.read());
  a += util::twice(a);
  a += util::ident<int>(a);
  a += util::ident(a);
  double d = static_cast<double>(a);
  auto f = [&](int z) { return helper(z); };
  std::printf("%d\n", a);
  return f(a) + (int)d;
}

void reset(Counter &c);
