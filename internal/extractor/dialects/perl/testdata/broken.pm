package Broken;

sub before {
  return 1;
}

sub oops {
  my $x = ;
  return +* 2;
}

sub after {
  return 3;
}
