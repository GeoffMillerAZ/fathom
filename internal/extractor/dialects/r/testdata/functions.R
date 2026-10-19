# Assignment forms, nesting and a multi-line signature.
add <- function(x, y = 1) {
  helper <- function(z) z + 1
  helper(x) + y
}

sq = function(x) x^2

counter <<- function() {
  total <<- total + 1
}

"quoted" <- function() NULL

print.point <- function(x,
                        digits = 3,
                        ...) {
  cat(format(x$v, digits = digits), "\n")
  invisible(x)
}

not_a_function <- 42
