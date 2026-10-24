# proxysql — provenance and divergences

LB-Q-proxysql, design `docs/design/language-breadth.md` §4 and §13. A doc-engine dialect (no `tags.scm`, no grammar, no
incumbent: a `proxysql.cnf` was unread). The fixtures are written for this dialect from the configuration file ProxySQL
documents (sysown/proxysql `etc/proxysql.cnf`, GPL-3.0; nothing is copied from its source), with reserved example hosts.

## The decode

`docdecode_libconfig.go`, the `libconfig` decode, written here in Go (no parser library, so no licence to pin): ProxySQL
reads its file with libconfig (hyperrealm), whose grammar is the one implemented. A group is a map, a list and an array
are sequences, a scalar is its text. A setting is `name = value` or `name : value`, ended or not by `;` or `,`; comments
are `#`, `//` and `/* */`; an `@include` line is dropped. A bare word that is neither a number nor a boolean is a syntax
error, as it is for ProxySQL (`method=GET`): the file reads as no entities.

## How a file is claimed

Manifest basenames `proxysql.cnf`, `proxysql.cfg`, globs `**/proxysql*.cnf`, `**/proxysql*.cfg`, `**/proxysql/**/*.cnf`,
and the doc spec's `content_sniff` (a ProxySQL group or `datadir` opens a line); both, through the gateway hub
(`fusion/gatewaydispatch.go`).

## What it models

- **Sections** (`ProxySQLSection`): every top-level group or list; **settings** (`ProxySQLSetting`): top-level scalars
  (`datadir`, `errorlog`); **variables** (`ProxySQLVariable`, `variable_decl`): a scalar of a group, `mysql_variables.threads`.
- **Servers** (`ProxySQLServer`, `target`), named `address:port@hostgroup`; **users** (`ProxySQLUser`), **query rules**
  (`ProxySQLQueryRule`) and **replication hostgroups** (`ProxySQLReplication`), each prefixed `mysql/` or `pgsql/`;
  **scheduler** entries (`ProxySQLScheduler`) and cluster **peers** (`ProxySQLPeer`).
- **Hostgroups** (`ProxySQLHostgroup`, `label`): a number a server names, then one a replication entry's writer or reader
  names, once per file (`unique`), mysql then pgsql. A server, a user's `default_hostgroup`, a rule's
  `destination_hostgroup` and `mirror_hostgroup` and a replication entry's writer and reader are in-file references to it
  (`declared_only`: a hostgroup no server or replication entry declares is no reference).

Every entity is `sensitive` (a file holds credentials). A variable is read whether or not ProxySQL knows it: the 17 the
oracle drops in `src/proxysql.cfg` belong to features the 3.0.9 release binary does not carry (`mcp_*`, `genai_*`).

## Not modelled (beyond the L2 target)

`address` as a cross-file service reference, `@include` as a file edge, and `restapi`/`mcp` entries.
