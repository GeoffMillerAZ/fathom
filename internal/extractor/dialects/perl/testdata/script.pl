#!/usr/bin/perl
use strict;
use warnings;

sub main {
  my @args = @_;
  report(scalar @args);
  return 0;
}

sub report {
  my ($n) = @_;
  print "args: $n\n";
}

exit main(@ARGV);
