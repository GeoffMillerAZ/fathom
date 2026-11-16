### Added

- IaC resource reads carry sorted read and unread input paths and value-free unknown reasons with closed deciding actions. The public library, resources command, and module/template review results preserve that evidence, including defaults-only files and incomplete inputs.

- Read evidence uses logical names for accepted POSIX, Windows drive and UNC absolute inputs, including unknown spans, while conserving legacy paths and distinguishing colliding input names.
