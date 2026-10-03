#include "lib/shapes.hpp"

// A test macro's last argument names the test; it is no type, even where a
// reached header defines a class of that name. The first argument of a fixture
// macro is a type.
#define TEST(suite, name) void suite##_##name()
#define TEST_F(fixture, name) void fixture##_##name()

TEST(shape_suite, Shape) { int unused = 0; (void)unused; }

TEST_F(Point, area) { int unused = 0; (void)unused; }
