import 'dart:core';

/// A documented class with annotations on its members.
@immutable
class Account {
  static int _next = 1;
  static const String prefix = 'acct-';
  late final String id;
  final List<String> tags;
  double? balance, limit;

  Account._internal(this.tags);

  Account() : tags = [] {
    id = '$prefix$_next';
  }

  @override
  String toString() => id;

  @Deprecated('use tags')
  List<String> get labels => tags;

  set labels(List<String> v) {
    tags
      ..clear()
      ..addAll(v);
  }

  external int digest();

  static Account of(String id) => Account()..id = id;

  int operator -(Account o) => 0;

  T convert<T extends Object>(T Function(Account) f) => f(this);
}

mixin Auditable<T> on Account {
  final List<T> log = [];
  int get count => log.length;
  void record(T entry) => log.add(entry);
}

extension type Cents(int value) implements int {
  int get dollars => value ~/ 100;
}

extension on int {
  int doubled() => this * 2;
}

enum Level with Comparable<Level> {
  low(1),
  high(3);

  const Level(this.weight);
  final int weight;

  @override
  int compareTo(Level other) => weight - other.weight;
  int get next => index + 1;
}

typedef Json = Map<String, Object?>;
typedef void Handler<E>(E event);
