# Names that are not plain identifiers.
.hidden <- function() 1

.onLoad <- function(libname, pkgname) {
  invisible()
}

`my fn` <- function(x) x

`%+%` <- function(a, b) paste(a, b)

'single' <- function() 2

x.y.z <- function() 3
