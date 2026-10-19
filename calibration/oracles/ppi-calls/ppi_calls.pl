#!/usr/bin/perl
# ppi_calls.pl -- tier-C Perl call oracle: PPI (an independent Perl parser library, pure Perl) over a checkout.
#
#   perl ppi_calls.pl <checkout> > out/perl-<subject>-calls.json
#
# Emits the neutral oracle shape (calibration/oracles/README.md): {"tool","version","tier","definitions":[],"calls":[{path,from,to}]}.
# A call is a (named sub, callee name) pair; a call no named sub encloses is skipped. A callee is:
#   - a Word followed by a parenthesised list:           name(...)  Pkg::name(...)
#   - a Word directly followed by a term:                name $x, ...   name "s"   name \@a   (a no-paren call; PPI cannot
#                                                          tell a predeclared sub from a bareword, so this is the heuristic part)
#   - the Word after an arrow:                           $obj->name  Class->name(...)  $obj->name(...)
# A Perl builtin or keyword (also as CORE::name) is no function callee, a method of the same name is (the list below is perlfunc 5.34's entries, minus its sub-headings, plus the
# 5.34+ syntax words try/catch/finally/defer/isa); the dialect's tags.scm excludes the same list.
use strict;
use warnings;
use File::Find ();
use JSON::PP ();
use PPI ();

my $root = shift or die "usage: ppi_calls.pl <checkout>\n";
$root =~ s{/+$}{};

my %SKIP = map { $_ => 1 } qw(
  __DATA__ __END__ __FILE__ __LINE__ __PACKAGE__ __SUB__ abs accept alarm and atan2 bind binmode bless break caller chdir
  chmod chomp chop chown chr chroot close closedir cmp connect continue cos crypt dbmclose dbmopen default defined delete
  die do dump each else elsif endgrent endhostent endnetent endprotoent endpwent endservent eof eq eval evalbytes exec
  exists exit exp fc fcntl fileno flock for foreach fork format formline ge getc getgrent getgrgid getgrnam gethostbyaddr
  gethostbyname gethostent getlogin getnetbyaddr getnetbyname getnetent getpeername getpgrp getppid getpriority
  getprotobyname getprotobynumber getprotoent getpwent getpwnam getpwuid getservbyname getservbyport getservent
  getsockname getsockopt given glob gmtime goto grep gt hex if index int ioctl join keys kill last lc lcfirst le length
  link listen local localtime lock log lstat lt m map mkdir msgctl msgget msgrcv msgsnd my ne next no not oct open
  opendir or ord our pack package pipe pop pos print printf prototype push q qq qr quotemeta qw qx rand read readdir
  readline readlink readpipe recv redo ref rename require reset return reverse rewinddir rindex rmdir s say scalar seek
  seekdir select semctl semget semop send setgrent sethostent setnetent setpgrp setpriority setprotoent setpwent
  setservent setsockopt shift shmctl shmget shmread shmwrite shutdown sin sleep socket socketpair sort splice split
  sprintf sqrt srand stat state study sub substr symlink syscall sysopen sysread sysseek system syswrite tell telldir
  tie tied time times tr truncate uc ucfirst umask undef unless unlink unpack unshift untie until use utime values vec
  wait waitpid wantarray warn when while write x xor y
  try catch finally defer isa
);

sub candidate {
    my ($abs, $rel) = @_;
    return 1 if $rel =~ /\.(?:pm|pl|t|psgi|plx|perl)$/i;
    return 0 if $rel =~ /\.[A-Za-z0-9]+$/;
    open my $fh, '<', $abs or return 0;
    my $first = <$fh>;
    close $fh;
    return defined $first && $first =~ /^#!.*\bperl\b/ ? 1 : 0;
}

my @files;
File::Find::find({ no_chdir => 1, wanted => sub {
    return unless -f $_;
    return if $File::Find::name =~ m{/\.git/};
    (my $rel = $File::Find::name) =~ s{^\Q$root\E/}{};
    push @files, $rel if candidate($File::Find::name, $rel);
} }, $root);
@files = sort @files;

sub next_sig { my $t = shift; $t->snext_sibling }
sub prev_sig { my $t = shift; $t->sprevious_sibling }

sub term_start {
    my $t = shift or return 0;
    return 1 if $t->isa('PPI::Token::Symbol') || $t->isa('PPI::Token::Quote') || $t->isa('PPI::Token::Number')
        || $t->isa('PPI::Token::QuoteLike') || $t->isa('PPI::Token::Magic') || $t->isa('PPI::Token::Cast');
    return 1 if $t->isa('PPI::Token::Operator') && $t->content eq '\\';
    return 0;
}

my (@calls, %seen, $parsed, $failed);
for my $rel (@files) {
    my $doc = PPI::Document->new("$root/$rel");
    if (!$doc) { $failed++; next; }
    $parsed++;
    my $words = $doc->find('PPI::Token::Word') || [];
    for my $w (@$words) {
        my $name = $w->content;
        next unless $name =~ /^[A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)*$/;
        # the enclosing named sub
        my $sub; my $p = $w;
        while ($p = $p->parent) { if ($p->isa('PPI::Statement::Sub') && !$p->isa('PPI::Statement::Scheduled') && defined $p->name && length $p->name && $p->block) { $sub = $p; last } }
        next unless $sub;
        # not a definition's own name, an include/package statement, a variable declaration's keyword
        my $stmt = $w->statement or next;
        next if $stmt->isa('PPI::Statement::Include') || $stmt->isa('PPI::Statement::Package');
        my $prev = prev_sig($w);
        my $next = next_sig($w);
        my $kind;
        (my $bare = $name) =~ s/^CORE:://;
        if ($prev && $prev->isa('PPI::Token::Operator') && $prev->content eq '->') {
            $kind = 'method';
        } else {
            next if $SKIP{$bare};
            next if $prev && $prev->isa('PPI::Token::Word') && $prev->content =~ /^(?:sub|package|use|no|require|new)$/;
            next if $prev && $prev->isa('PPI::Token::Cast');
            next if $w->parent->isa('PPI::Structure::Subscript') && !$next;
            next if $next && $next->isa('PPI::Token::Operator') && $next->content =~ /^(?:=>|->)$/;
            next if $next && $next->isa('PPI::Token::Operator') && $next->content eq '::';
            if ($next && $next->isa('PPI::Structure::List')) { $kind = 'paren' }
            elsif (term_start($next)) { $kind = 'ambiguous' }
            else { next }
        }
        my $key = join "\0", $rel, $sub->name, $name;
        next if $seen{$key}++;
        push @calls, { path => $rel, from => $sub->name, to => $name };
    }
}
@calls = sort { $a->{path} cmp $b->{path} || $a->{from} cmp $b->{from} || $a->{to} cmp $b->{to} } @calls;
print JSON::PP->new->canonical->pretty->encode({
    tool => 'ppi-calls', version => "PPI $PPI::VERSION", tier => 'C',
    definitions => [], calls => \@calls,
});
printf STDERR "files=%d parsed=%d failed=%d calls=%d\n", scalar(@files), $parsed // 0, $failed // 0, scalar(@calls);
