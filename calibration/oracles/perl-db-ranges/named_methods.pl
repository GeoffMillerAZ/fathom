package MethodProbe;
use strict;
use warnings;

BEGIN {
    require Socket;
    Socket->import(qw(PF_INET SOCK_STREAM sockaddr_in inet_aton));
    socket(my $socket, Socket::PF_INET(), Socket::SOCK_STREAM(), getprotobyname('tcp')) or die "socket:$!";
    my $connected = connect($socket, sockaddr_in(1, inet_aton('127.0.0.1')));
    die 'BEGIN network unexpectedly connected' if $connected;
    die "BEGIN network was not denied with EPERM: $!" unless 0 + $! == 1;
}

sub first { return 1 }
sub second { return 2 }
sub method { return 3 }

sub caller {
    first();
    MethodProbe::second();
    my $object = bless {}, 'MethodProbe';
    $object->method();
    MethodProbe->method();
    my $dynamic = 'method';
    $object->$dynamic();
    my $closure = sub {
        $object
            ->closure_only();
    };
    return $closure;
}

sub closure_only { return 4 }
sub native_constant () { 7 }
sub constant_caller { native_constant(); }

die 'ordinary body must not execute';
1;
