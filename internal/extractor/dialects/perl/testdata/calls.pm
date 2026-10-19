package Calls;
use strict;

sub helper { return 1 }

sub plain {
  helper();
  helper(2);
  Other::Mod::run(1, 2);
  print("x");
  log_it "message", 3;
  my $code = \&helper;
  &$code(1);
  return;
}

sub methods {
  my ($self) = @_;
  $self->init(1);
  Calls->new->configure(a => 1)->run;
  my $name = "dyn";
  $self->$name();
  __PACKAGE__->boot;
}

sub in_closure {
  my @list = map { helper($_) } (1, 2);
  my $cb = sub { callback(1) };
  $cb->();
}

top_level_call();
