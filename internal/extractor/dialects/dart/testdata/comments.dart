// A comment between a signature and its `=>` body (LB-F247): the callable is
// still a definition, its range reaches the body and its calls are its own.
class Channel {
  static String signKey(String key)
      // One comment line.
      // A second comment line.
      =>
      encode(key);

  int size()
      // Only one.
      => count();

  int plain() => 1;
}

String top(String k)
    // a note
    => decode(k);

String top2(String k)
    // a note
    // another
    => decode(k);
