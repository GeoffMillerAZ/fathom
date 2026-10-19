use strict;
use warnings;
use Test::More tests => 2;

sub fixture {
  return { name => 'x' };
}

ok(fixture()->{name} eq 'x', 'name');
is(scalar(keys %{ fixture() }), 1, 'keys');
