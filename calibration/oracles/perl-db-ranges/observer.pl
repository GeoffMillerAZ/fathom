package DB;
use strict;
use warnings;

# Loaded by PERL5DB before the subject is compiled. Never delegate to a
# debugger or call a subject subroutine. Perl itself populates %DB::sub.
sub DB { }

CHECK {
    require JSON::PP;
    my $out = $ENV{FATHOM_DB_RANGE_OUT} or die "missing range output\n";
    open my $fh, '>', $out or die "range output open: $!\n";
    print {$fh} JSON::PP->new->canonical->encode(\%DB::sub)
        or die "range output write: $!\n";
    close $fh or die "range output close: $!\n";
}

1;
