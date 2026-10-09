// dialects/cpp/testdata/members.hpp — data members named after their class,
// enumerators after their enum, namespace-scope variables after their
// namespace (LB-F295); member function declarations, locals and parameters
// are none.
#pragma once
#include <cstddef>

namespace geo {

constexpr int kMaxPoints = 64;
extern const char *names[];
static double (*on_scale)(double);

struct point {
  int x, y;
  static const int dims = 2;
  point *next;
  double &weight_ref;
  char tag[8];
  void (*visit)(point &);
  unsigned dirty : 1;
  int area() const;
  int perimeter() const { return x + y; }
  union {
    int as_int;
    float as_real;
  };
};

class shape {
 public:
  enum class kind { circle, square };
  enum { none = 0, filled = 1 };
  explicit shape(kind k) : kind_(k) {}

 private:
  kind kind_;
  std::size_t sides_ = 0;
};

inline int count(const point &p) {
  int local = p.x;
  static int calls;
  return local + calls;
}

}  // namespace geo
