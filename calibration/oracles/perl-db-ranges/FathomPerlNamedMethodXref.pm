package FathomPerlNamedMethodXref;
use strict;
use warnings;
use B qw(class comppadlist SVf_POK);
use B::Xref 1.07;

our $VERSION = '0.01-probe';

# Private adapter for stock B::Xref 1.07. Install before O's CHECK walk.
# Keep Xref's own CV and main walk, COP state, direct-call handling and output.
my $original_process = \&B::Xref::process;
my $original_entersub = \&B::Xref::pp_entersub;
my $original_xref_cv = \&B::Xref::xref_cv;
my $original_xref_main = \&B::Xref::xref_main;
our $padlist;
my $suppress_for;
our $inside_matching_entersub = 0;

sub _literal {
    my ($op) = @_;
    my $sv = $op->meth_sv;
    if (!$$sv) {
        return unless $padlist && $$padlist;
        my @levels = $padlist->ARRAY;
        return unless @levels > 1 && $levels[1] && ${$levels[1]};
        my @values = $levels[1]->ARRAY;
        my $index = $op->targ;
        return unless defined($index) && $index >= 0 && $index < @values;
        $sv = $values[$index];
    }
    return unless $sv && $$sv && class($sv) ne 'SPECIAL';
    return unless $sv->FLAGS & SVf_POK;
    my $name = $sv->PV;
    return unless defined($name) && length($name) && $name !~ /\0/;
    return $name;
}

sub _method_named {
    my ($op) = @_;
    my $name = _literal($op);
    return unless defined $name;
    my $next = $op->next;
    return unless $$next && $next->name eq 'entersub';
    # Method name only. Neither the receiver type nor dispatch target is known.
    $original_process->(['(method)', '->', $name], 'meth');
    $suppress_for = $$next;
}

{
    no warnings 'redefine';
    *B::Xref::xref_cv = sub {
        my ($cv) = @_;
        local $padlist = $cv->PADLIST;
        $original_xref_cv->(@_);
    };
    *B::Xref::xref_main = sub {
        local $padlist = comppadlist;
        $original_xref_main->(@_);
    };
    *B::Xref::process = sub {
        my ($var, $event) = @_;
        return if $inside_matching_entersub && $event eq 'subused';
        $original_process->(@_);
    };
    *B::Xref::pp_entersub = sub {
        my ($op) = @_;
        local $inside_matching_entersub =
            defined($suppress_for) && $suppress_for == $$op;
        undef $suppress_for if $inside_matching_entersub;
        $original_entersub->(@_);
    };
    *B::Xref::pp_method_named = \&_method_named;
    *B::Xref::pp_method_redir = \&_method_named;
    *B::Xref::pp_method_redir_super = \&_method_named;
    *B::Xref::pp_method_super = \&_method_named;
}

1;
