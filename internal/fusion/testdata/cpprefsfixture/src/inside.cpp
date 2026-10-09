#include "lib/shapes.hpp"

namespace lib {
int inside(Point p, detail::Scalar s) { return p.x + static_cast<int>(s); }
}  // namespace lib
