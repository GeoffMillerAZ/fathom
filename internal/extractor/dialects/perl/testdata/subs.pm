package Shop::Cart;
use strict;
use warnings;

sub new {
  my ($class, %args) = @_;
  return bless {%args}, $class;
}

sub total($self, $tax = 0.2) {
  return $self->{sum} * (1 + $tax);
}

sub proto($$) { return 1 }

sub counter :lvalue {
  $Shop::Cart::count;
}

sub forward;

sub Shop::Util::trim {
  my $s = shift;
  $s =~ s/^\s+|\s+$//g;
  return $s;
}

my $anon = sub { 42 };

package Shop::Order;

sub place {
  return 1;
}

1;
