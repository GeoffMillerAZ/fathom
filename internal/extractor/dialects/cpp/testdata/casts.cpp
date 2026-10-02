/* dialects/cpp/testdata/casts.cpp — a functional cast to a builtin or a
 * standard integer type is no call (LB-F117): unsigned(t), short(4),
 * std::size_t(n), uint32_t(a) name no function; a call of a function and a
 * class constructor written T(x) stay calls. */

#include <cstdint>

struct Widget {
  explicit Widget(int v) : v_(v) {}
  int v_;
};

int widen(int v) { return v; }

unsigned mix(unsigned t, long n) {
  auto a = unsigned(t) | short(4);
  auto b = std::size_t(n) + uint32_t(a) + char(65) + int(t);
  Widget w = Widget(widen(1));
  return a + b + w.v_;
}
