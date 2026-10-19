# Calls: plain, namespaced, nested in an anonymous function, and a call no function encloses.
library(stats)

load_all <- function(paths) {
  parts <- lapply(paths, function(p) readLines(p))
  dplyr::bind_rows(parts)
  utils:::head(parts, 1)
  obj$method(1)
}

report <- function(df) {
  load_all(df)
  stats::sd(df$x)
  load_all(df)
}
