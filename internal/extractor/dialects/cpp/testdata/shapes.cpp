/* dialects/cpp/testdata/shapes.cpp — coverage fixture: every required kind,
 * containment (a method in a class in a namespace), templates, out-of-class
 * definitions, operators, a multi-line signature, and extern "C". */

#define MAX_ITEMS 64

#define SQUARE(x) ((x) * (x))

namespace geo {

namespace detail {
int scale(int v) { return v * 2; }
}

struct Point {
  int x;
  int y;
};

union Cell {
  int as_int;
  float as_float;
};

enum class Mode : int { Fast, Slow };

enum Color { Red, Green };

typedef unsigned long Id;
typedef int (*Callback)(int);
using PointList = Point *;

class Shape {
public:
  Shape() = default;
  explicit Shape(int sides) : sides_(sides) {}
  virtual ~Shape() {}
  int sides() const { return sides_; }
  Shape &operator=(const Shape &other) { return *this; }
  bool operator==(const Shape &other) const { return sides_ == other.sides_; }
  virtual double area() const;
  static Shape unit();

  struct Edge {
    int len;
    int measure() { return len; }
  };

  enum Kind { Flat, Round };

private:
  int sides_;
};

template <typename T>
class Box {
public:
  T &get();
  template <typename U> void put(U u) { value_ = u; }

private:
  T value_;
};

template <typename T>
T largest(T a, T b) { return a > b ? a : b; }

} // namespace geo

namespace outer::inner {
int deep() { return 1; }
}

namespace {
int hidden() { return 0; }
}

double geo::Shape::area() const { return 0.0; }

geo::Shape geo::Shape::unit() { return Shape(1); }

template <typename T>
T &geo::Box<T>::get() { return value_; }

int *make_ptr(int n) { return new int(n); }

int &pick(int &a,
          int &b,
          bool first) {
  return first ? a : b;
}

extern "C" {
void c_entry(void) {}
}

template <>
int geo::largest<int>(int a, int b) { return a; }

class Forward;
struct Pending;
void declared_only(int);
