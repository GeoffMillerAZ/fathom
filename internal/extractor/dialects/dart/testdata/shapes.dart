library shapes;

import 'dart:math';

typedef Callback = void Function(int value);
typedef int Compare(Object a, Object b);

const int maxSides = 12;
final String label = 'shape', other = 'x';
int counter = 0;

int get total => counter;

set total(int value) {
  counter = value;
}

int area(int w, int h) => w * h;

Future<List<T>> collect<T>(
  Iterable<T> items, {
  int limit = 10,
}) async {
  int helper(int x) {
    return x + 1;
  }
  return items.take(helper(limit)).toList();
}

abstract class Shape<T> extends Base with Mixin implements Comparable<Shape> {
  static const int sides = 4;
  final String name;
  int _count = 0, extra = 1;

  Shape(this.name);

  Shape.named(String n) : name = n, super();

  const Shape.unit() : name = 'unit';

  factory Shape.make() => Circle();

  factory Shape.alias() = Circle;

  String get title => name;

  set title(String v) {}

  int operator +(int o) => o;

  bool operator ==(Object other) => true;

  double area();

  void run(
    int a, {
    required String b,
    int c = 1,
  }) {
    print(a);
  }
}

sealed class Node {}

mixin class Walker {
  void walk() {}
}

mixin Logger on Base implements Walker {
  void log(String m) => print(m);
}

class Both = Base with Logger;

extension StringX on String {
  int twice() => length * 2;
}

enum Color {
  red,
  green(1);

  const Color([this.code = 0]);
  final int code;

  String describe() => name;
}
