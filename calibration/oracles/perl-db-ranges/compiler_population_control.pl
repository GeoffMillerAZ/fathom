package CompilerPopulationControl;
use strict;
use warnings;

sub native_constant () { 7 }
sub constant_caller { native_constant() }
sub literal_caller { 7 }
sub target { 3 }
sub closure_factory { return sub { target() } }

die 'ordinary body must never execute';

CHECK {
    require B;
    require B::Concise;
    my %seen;
    my @anonymous;
    my $walk;
    $walk = sub {
        my ($cv) = @_;
        return if $seen{$$cv}++;
        my $pad = $cv->PADLIST;
        return unless $$pad && B::class($pad) eq 'PADLIST';
        my @levels = $pad->ARRAY;
        return unless @levels > 1;
        for my $value ($levels[1]->ARRAY) {
            next unless $$value && B::class($value) eq 'CV';
            push @anonymous, $value;
            $walk->($value);
        }
    };
    $walk->(B::svref_2object(\&closure_factory));
    print "ANONYMOUS_CVS=", scalar(@anonymous), "\n";
    for my $pair ([constant_caller => \&constant_caller], [literal_caller => \&literal_caller]) {
        print "CONTROL=$pair->[0]\n";
        B::Concise::compile('-basic', $pair->[1])->();
    }
    for my $cv (@anonymous) {
        print "CONTROL=anonymous\n";
        B::Concise::compile('-basic', $cv->object_2svref)->();
    }
}
