; source: written for this dialect from the tree-sitter-perl 2.0.0 node names (tree-sitter-perl/tree-sitter-perl @
; 4586b3f1fc6f8b80cca6ceb23ba863ae986eeae8, MIT); the grammar ships no queries/tags.scm, and no third-party Perl
; tags query was consulted. A package is its `package` statement (the block form `package Foo { ... }` also contains
; its subs, so they are qualified by it; the statement form `package Foo;` spans only itself). A sub, a `class`
; and a `method` (the 5.38 class feature) are named as written (`Pkg::name` for a qualified sub) and captured
; with @body, the block, so a signature stops before it and holds the prototype, attributes and signature.
; A forward declaration (`sub name;`) has no body and is no definition. Calls are a named function (`f(1)`,
; `Pkg::f(1)`, `f 1`) or a method (`$obj->m`, `Class->m`); a callee that is not a name (`&$code(1)`, `$obj->$name`)
; has no name to record and is left out, and so is a function named like a Perl builtin or keyword, with or
; without CORE:: (a method of that name is kept; the list is perlfunc 5.34's entries plus
; try/catch/finally/defer/isa, the one calibration/oracles/ppi-calls/ppi_calls.pl skips). Not extracted, on purpose:
; anonymous subs, typeglob assignments, `use constant`, variables, labels, formats.

(package_statement
  name: (package) @name) @definition.package

(class_statement
  name: (package) @name) @definition.class

(subroutine_declaration_statement
  name: (bareword) @name
  body: (block) @body) @definition.function

(method_declaration_statement
  name: (bareword) @name
  body: (block) @body) @definition.method

(function_call_expression
  function: (function) @name
  (#match? @name "^[A-Za-z_]")
  (#not-match? @name "^(?:CORE::)?(?:__DATA__|__END__|__FILE__|__LINE__|__PACKAGE__|__SUB__|abs|accept|alarm|and|atan2|bind|binmode|bless|break|caller|catch|chdir|chmod|chomp|chop|chown|chr|chroot|close|closedir|cmp|connect|continue|cos|crypt|dbmclose|dbmopen|default|defer|defined|delete|die|do|dump|each|else|elsif|endgrent|endhostent|endnetent|endprotoent|endpwent|endservent|eof|eq|eval|evalbytes|exec|exists|exit|exp|fc|fcntl|fileno|finally|flock|for|foreach|fork|format|formline|ge|getc|getgrent|getgrgid|getgrnam|gethostbyaddr|gethostbyname|gethostent|getlogin|getnetbyaddr|getnetbyname|getnetent|getpeername|getpgrp|getppid|getpriority|getprotobyname|getprotobynumber|getprotoent|getpwent|getpwnam|getpwuid|getservbyname|getservbyport|getservent|getsockname|getsockopt|given|glob|gmtime|goto|grep|gt|hex|if|index|int|ioctl|isa|join|keys|kill|last|lc|lcfirst|le|length|link|listen|local|localtime|lock|log|lstat|lt|m|map|mkdir|msgctl|msgget|msgrcv|msgsnd|my|ne|next|no|not|oct|open|opendir|or|ord|our|pack|package|pipe|pop|pos|print|printf|prototype|push|q|qq|qr|quotemeta|qw|qx|rand|read|readdir|readline|readlink|readpipe|recv|redo|ref|rename|require|reset|return|reverse|rewinddir|rindex|rmdir|s|say|scalar|seek|seekdir|select|semctl|semget|semop|send|setgrent|sethostent|setnetent|setpgrp|setpriority|setprotoent|setpwent|setservent|setsockopt|shift|shmctl|shmget|shmread|shmwrite|shutdown|sin|sleep|socket|socketpair|sort|splice|split|sprintf|sqrt|srand|stat|state|study|sub|substr|symlink|syscall|sysopen|sysread|sysseek|system|syswrite|tell|telldir|tie|tied|time|times|tr|truncate|try|uc|ucfirst|umask|undef|unless|unlink|unpack|unshift|untie|until|use|utime|values|vec|wait|waitpid|wantarray|warn|when|while|write|x|xor|y)$")) @reference.call

(ambiguous_function_call_expression
  function: (function) @name
  (#match? @name "^[A-Za-z_]")
  (#not-match? @name "^(?:CORE::)?(?:__DATA__|__END__|__FILE__|__LINE__|__PACKAGE__|__SUB__|abs|accept|alarm|and|atan2|bind|binmode|bless|break|caller|catch|chdir|chmod|chomp|chop|chown|chr|chroot|close|closedir|cmp|connect|continue|cos|crypt|dbmclose|dbmopen|default|defer|defined|delete|die|do|dump|each|else|elsif|endgrent|endhostent|endnetent|endprotoent|endpwent|endservent|eof|eq|eval|evalbytes|exec|exists|exit|exp|fc|fcntl|fileno|finally|flock|for|foreach|fork|format|formline|ge|getc|getgrent|getgrgid|getgrnam|gethostbyaddr|gethostbyname|gethostent|getlogin|getnetbyaddr|getnetbyname|getnetent|getpeername|getpgrp|getppid|getpriority|getprotobyname|getprotobynumber|getprotoent|getpwent|getpwnam|getpwuid|getservbyname|getservbyport|getservent|getsockname|getsockopt|given|glob|gmtime|goto|grep|gt|hex|if|index|int|ioctl|isa|join|keys|kill|last|lc|lcfirst|le|length|link|listen|local|localtime|lock|log|lstat|lt|m|map|mkdir|msgctl|msgget|msgrcv|msgsnd|my|ne|next|no|not|oct|open|opendir|or|ord|our|pack|package|pipe|pop|pos|print|printf|prototype|push|q|qq|qr|quotemeta|qw|qx|rand|read|readdir|readline|readlink|readpipe|recv|redo|ref|rename|require|reset|return|reverse|rewinddir|rindex|rmdir|s|say|scalar|seek|seekdir|select|semctl|semget|semop|send|setgrent|sethostent|setnetent|setpgrp|setpriority|setprotoent|setpwent|setservent|setsockopt|shift|shmctl|shmget|shmread|shmwrite|shutdown|sin|sleep|socket|socketpair|sort|splice|split|sprintf|sqrt|srand|stat|state|study|sub|substr|symlink|syscall|sysopen|sysread|sysseek|system|syswrite|tell|telldir|tie|tied|time|times|tr|truncate|try|uc|ucfirst|umask|undef|unless|unlink|unpack|unshift|untie|until|use|utime|values|vec|wait|waitpid|wantarray|warn|when|while|write|x|xor|y)$")) @reference.call

(method_call_expression
  method: (method) @name
  (#match? @name "^[A-Za-z_]")) @reference.call
