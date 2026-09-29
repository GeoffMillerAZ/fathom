library app;

import 'dart:async';
import 'package:acme/util.dart';
import 'package:acme/src/model.dart' as model;
import 'src/helper.dart' show Helper, assist;
import '../shared/strings.dart' hide Secret;
import "widgets/card.dart" deferred as card;
import 'store.dart' if (dart.library.io) 'store_io.dart';
import 'a.dart' as a show Only;

export 'src/api.dart';
part 'app.g.dart';

class App {
  void run() {
    Helper().go();
    model.Model.load();
    assist();
  }
}
