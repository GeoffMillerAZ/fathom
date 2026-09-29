# Procedure definitions and the shapes of their headers.

proc greet {name} {
    puts "hello, $name"
}

proc add {a b} {
    return [expr {$a + $b}]
}

# Defaults, and a trailing args.
proc connect {host {port 8080} {timeout 30} args} {
    return [list $host $port $timeout $args]
}

# A header that spans lines.
proc render {
    template
    {vars {}}
    {escape 1}
} {
    set out $template
    return $out
}

# An empty procedure, and two on one line.
proc noop {} {}
proc twice {x} { return [expr {$x * 2}] }

# Qualified names, absolute and relative.
proc ::util::pad {s n} {
    return [string range $s 0 $n]
}
proc util::trim {s} {
    return [string trim $s]
}

# Defined conditionally, and inside another procedure.
if {[info commands legacy] eq ""} {
    proc legacy {} {
        return 1
    }
}

proc installer {} {
    proc helper {} {
        return 2
    }
    helper
}

# A name built at run time is not a definition.
proc $dynamic {x} {
    return $x
}
