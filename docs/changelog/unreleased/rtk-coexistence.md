### Changed

- RTK's own tools and controls remain RTK commands in fathom's shell checks; `rtk test`, `rtk err` and `rtk summary` expose the command they wrap. Readouts leave RTK output unchanged while test runs are still recorded and normal test failure context remains available.
