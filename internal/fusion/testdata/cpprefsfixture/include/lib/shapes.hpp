#pragma once
namespace lib {
struct Point { int x; int y; };
class Shape {
 public:
  virtual ~Shape() = default;
  virtual int area() const = 0;
};
namespace detail {
using Scalar = long;
}  // namespace detail
#define LIB_VERSION 3
}  // namespace lib
