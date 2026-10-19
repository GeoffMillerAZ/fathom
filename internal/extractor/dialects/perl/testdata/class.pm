use v5.38;
use feature 'class';
no warnings 'experimental::class';

class Point {
  field $x :param = 0;
  field $y :param = 0;

  method move($dx, $dy) {
    $x += $dx;
    $y += $dy;
    return $self;
  }

  method coords { return ($x, $y) }
}

class Point3D :isa(Point) {
  field $z :param = 0;
  method lift($dz) { $z += $dz }
}
