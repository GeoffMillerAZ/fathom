### Added

- ProxySQL configuration (`proxysql.cnf`, `proxysql.cfg`, `proxysql*.cnf`) extraction at L2 (L1 good, L2 excellent), measured against ProxySQL 3.0.9's own load of sysown/proxysql's configuration files: sections, variables, servers, users, query rules, replication hostgroups, scheduler entries and cluster peers, and the hostgroup each server, user, rule and replication entry names as an in-file reference. A file libconfig refuses (a bare word where a quoted string belongs) reads as no entities.
