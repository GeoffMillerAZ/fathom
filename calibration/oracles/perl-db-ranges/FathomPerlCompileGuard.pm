package DB;
use strict;
use warnings;

# Loaded by PERL5DB for range collection or -M for Xref. Never delegate to a
# debugger or call a subject subroutine. Perl itself populates %DB::sub.
sub DB { }

BEGIN {
    my $separator = $ENV{FATHOM_PATH_SEP} or die "missing include-path separator\n";
    my $paths = $ENV{FATHOM_ALLOWED_INC} or die "missing allowed include paths\n";
    my @allowed = split /\Q$separator\E/, $paths;
    @INC = @allowed;
}

CHECK {
    require JSON::PP;
    require Cwd;
    my $separator = $ENV{FATHOM_PATH_SEP} or die "missing include-path separator\n";
    my $roots = $ENV{FATHOM_ALLOWED_ROOTS} or die "missing allowed module roots\n";
    my @roots = map { Cwd::abs_path($_) // die "invalid module root\n" }
                split /\Q$separator\E/, $roots;
    my $observer = Cwd::abs_path(__FILE__);
    for my $module (values %INC) {
        next if !defined($module) || ref($module);
        my $path = Cwd::abs_path($module);
        die "module path cannot be resolved\n" unless defined $path;
        next if defined($observer) && $path eq $observer;
        my $allowed = 0;
        for my $root (@roots) {
            $allowed = 1 if $path eq $root || index($path, $root . '/') == 0;
        }
        die "module loaded outside C-R9 roots\n" unless $allowed;
    }
    my $out = $ENV{FATHOM_DB_RANGE_OUT} or die "missing range output\n";
    open my $fh, '>', $out or die "range output open: $!\n";
    print {$fh} JSON::PP->new->canonical->encode(\%DB::sub)
        or die "range output write: $!\n";
    close $fh or die "range output close: $!\n";
}

1;
