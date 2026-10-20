### Fixed

- A managed daemon started by hand no longer keeps launchd's or systemd's own instance down: the daemon record and start row say whether a supervisor runs it, a relaunch that finds an unsupervised same-label holder exits 75 (`port_held_unsupervised`, never counted by the breaker) so the supervisor takes the port over once it is free, and the hand start and the daemon-down text name the supervisor's restart command (O1363).
