import 'package:acme/src/model.dart';
import 'package:flutter/material.dart';
import 'src/util.dart';
import 'src/net.dart' as net;

void run() {
  final m = Model();
  m.save();
  print(format(label(m)));
  net.fetch();
}
