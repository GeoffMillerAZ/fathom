# Namespaces, nested and qualified, and the procedures inside them.
package require Tcl 8.6

namespace eval ::app {
    variable count 0
    namespace export init run

    proc init {} {
        variable count
        set count 0
    }

    proc run {name {mode fast}} {
        variable count
        incr count
        ::app::util::trim $name
    }

    namespace eval util {
        proc trim {s} {
            return [string trim $s]
        }
    }

    namespace ensemble create
}

namespace eval app::config {
    proc load {path} {
        return [read_file $path]
    }
}

# An empty namespace is one; a namespace named at run time is not.
namespace eval empty {}
namespace eval $runtime { proc dynamic {} {} }
namespace import ::app::*

proc ::app::extra {} {
    return 1
}
