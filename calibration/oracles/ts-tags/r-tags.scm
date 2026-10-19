; source: r-lib/tree-sitter-r queries/tags.scm
; @ 346d3707b8c9301f1051e8f6e32666e67529f7d2 (v1.3.0, MIT), verbatim below this
; header. It is the grammar's own upstream tags query, run here by py-tree-sitter
; (the C runtime) as the tier-C L2 call oracle (design section 1.5 D5).

(binary_operator
    lhs: (identifier) @name
    operator: "<-"
    rhs: (function_definition)
) @definition.function

(binary_operator
    lhs: (identifier) @name
    operator: "="
    rhs: (function_definition)
) @definition.function

(binary_operator
    lhs: (string) @name
    operator: "<-"
    rhs: (function_definition)
) @definition.function

(binary_operator
    lhs: (string) @name
    operator: "="
    rhs: (function_definition)
) @definition.function

(call
    function: (identifier) @name
) @reference.call

(call
    function: (namespace_operator
        rhs: (identifier) @name
    )
) @reference.call
