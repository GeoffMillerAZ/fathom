# Which definition owns each command call.

proc plain {} {
    helper
    helper
    other 1 2
}

proc qualified {} {
    ::app::util::trim x
    app::run y
}

proc substituted {x} {
    set y [compute $x]
    lappend out [format "%d" [normalize $y]]
    puts "value: [describe $y]"
}

proc control {items} {
    foreach item $items {
        visit $item
    }
    if {[ready]} {
        go
    } else {
        wait
    }
    while {[more]} {
        step
    }
    catch {risky $items} err
    try {
        attempt
    } finally {
        cleanup
    }
}

# A callee that is not a plain name has no name to record.
proc dynamic {obj cmds} {
    $obj method arg
    [lindex $cmds 0] arg
}

# A call inside a nested procedure belongs to the nested one.
proc outer {} {
    proc inner {} {
        deep
    }
    shallow
}

# A call outside any procedure has no owner.
top_level
