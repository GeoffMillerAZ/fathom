namespace go acme.audit

include "shared.thrift"

struct Stamp {
  1: shared.Timestamp at,
  2: string by
}

service Audited {
  Stamp last()
}
