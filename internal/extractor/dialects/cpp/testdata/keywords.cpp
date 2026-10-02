/* dialects/cpp/testdata/keywords.cpp — operators and keywords written like
 * calls are no calls (LB-F117): noexcept(noexcept(...)), sizeof(...),
 * alignof(...), decltype(...) and a standard macro, assert(...); the calls
 * written inside them stay calls. A call through the `template`
 * disambiguator (Factory::template create<T>(...), box.template get<0>()) is
 * a call to the name after it. */

#include <cassert>
#include <utility>

int probe(int v) { return v; }

template <typename T>
auto relay(T &&v) noexcept(noexcept(probe(std::forward<T>(v))))
    -> decltype(probe(std::forward<T>(v))) {
  assert(sizeof(v) > 0 && alignof(T) > 0);
  return probe(std::forward<T>(v));
}

template <typename Factory, typename Box>
int build(int n, Box &box) {
  return Factory::template create<int>(n) + box.template get<0>();
}
