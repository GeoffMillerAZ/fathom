# Backslash lambdas assigned to a name, an S3 method and a pipe.
square <- \(x) x^2

summary.point <- function(object, ...) {
  object$v |> sqrt()
}

compose <- function(f, g) function(x) g(f(x))
