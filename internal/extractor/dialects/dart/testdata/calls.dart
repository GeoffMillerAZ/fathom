import 'dart:async';

class Client {
  Client(this.base);

  final Uri base;

  Client.local() : base = Uri.parse('http://localhost') {
    init();
  }

  factory Client.create() {
    return Client(Uri.parse('x'));
  }

  Future<void> run(List<String> xs) async {
    plain(1);
    helper.method(2).chain(3);
    maybe?.safe();
    force!.bang();
    Client.local();
    Client.create().send();
    new Client(base);
    new Client.local();
    const Point(1, 2);
    const Point.origin();
    xs
      ..add('a')
      ..remove('b');
    super.close();
    this.send();
    generic<int>(4);
    await Future.delayed(const Duration(seconds: 1));
    (callback)();
    curry()();
    items[0].first();
    final total = Math.sum(xs.map((x) => convert(x)).toList());
    var f = () => inner();
  }

  void send() {
    void nested() {
      deep();
    }
    nested();
  }
}

int top() => Client(Uri()).base.hashCode + other();

final registry = Registry.build(seed());

class Stream1 {
  final ctrl = StreamController<int>.broadcast(sync: true);
  Future<void> wait() => Future<void>.delayed(Duration.zero);
  void listen(Stream s) {
    s..stream.listen(sink.add);
  }
}
