# A file with a syntax error: the definitions around it survive.
proc before {} {
    return 1
}

namespace eval ok {
    proc inside {} { return 2 }
}

proc broken {x} {
    return [expr {$x +}
}}

proc after {} {
    return 4
}
