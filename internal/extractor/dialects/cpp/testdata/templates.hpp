/* dialects/cpp/testdata/templates.hpp — header-style shapes: class templates,
 * an explicit specialisation, an alias template, an out-of-class template
 * member, inheritance, defaulted members and attributes. */
#pragma once

#include <memory>
#include <vector>

namespace ds {

template <typename K, typename V>
class Map {
public:
  using Entry = std::pair<K, V>;
  Map() {}
  V &at(const K &key);
  bool insert(const K &key, const V &value) { return true; }
  template <typename F> void for_each(F fn) const { fn(1); }

private:
  std::vector<Entry> entries_;
};

template <typename K, typename V>
V &Map<K, V>::at(const K &key) { return entries_.front().second; }

template <>
class Map<int, int> {
public:
  int total() const { return 0; }
};

template <typename T>
using Vec = std::vector<T>;

template <typename T>
struct Holder {
  T value;
  T get() const { return value; }
};

class Base {
public:
  virtual ~Base() = default;
  virtual void run() = 0;
};

class Derived final : public Base {
public:
  void run() override {}
};

inline constexpr int kLimit = 8;

constexpr int square(int v) noexcept { return v * v; }

[[nodiscard]] inline bool ok() { return true; }

} // namespace ds
