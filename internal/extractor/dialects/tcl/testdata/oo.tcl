# TclOO classes and their methods.
oo::class create Animal {
    variable name sound

    constructor {n s} {
        set name $n
        set sound $s
    }

    method speak {} {
        return [my format $sound]
    }

    # The argument list is read as a script here too: `prefix` and `suffix` come out as calls.
    method describe {
        {prefix ""}
        {suffix ""}
    } {
        return "$prefix$name$suffix"
    }

    destructor {
        cleanup
    }
}

oo::class create Dog {
    superclass Animal
    method fetch {} {
        return [my speak]
    }
}

namespace eval ::zoo {
    oo::class create Keeper {
        # A plain argument list reads as a command in this grammar, so `animal` is a call.
        method feed {animal} {
            feeder
        }
    }
}

oo::class create ::zoo::Cage {
    method open {} {}
}

# Not definitions here: a class named at run time, and a method added from outside.
oo::class create $dynamic { method hidden {} {} }
oo::define Animal method added {} {}
