### Fixed

- `doctor --brief` (the Setup hook) no longer sends a daemon read after it returns. Its capabilities row built the console model through a detached rebuild, and when that rebuild reached the daemon after the hook's deadline it sent `list_services` with no `X-Fathom-Deadline`. Doctor now builds the model inline with its reads bound to the probe's deadline.
