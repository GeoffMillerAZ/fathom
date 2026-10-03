#include "lib/shapes.hpp"

namespace app {
class Canvas {
 public:
  int draw(const lib::Shape& s, lib::Point p) { return s.area() + p.x + LIB_VERSION; }
};
int scale(lib::detail::Scalar v) { return static_cast<int>(v); }
}  // namespace app
