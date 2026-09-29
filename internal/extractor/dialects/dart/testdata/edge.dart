import 'dart:js_interop';

@JS('fetch')
external JSPromise<Object> fetchIt(String input);

external void note();

void plain() {}

abstract class Base {
  abstract final Uri url;
  abstract int count;

  static String sign(String key) => key.trim();

  void quiet() {}
}
