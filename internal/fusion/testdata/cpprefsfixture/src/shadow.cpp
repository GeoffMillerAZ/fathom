#include "lib/shapes.hpp"

namespace local {
struct Point { int q; };
int use(Point p) { return p.q; }
template <typename T>
T twice(T v) { return v + v; }
}  // namespace local

int std_user(std::size_t n) { return static_cast<int>(n); }
