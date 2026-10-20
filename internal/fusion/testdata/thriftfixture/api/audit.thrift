include "common/types.thrift"

struct Stamp {
  1: types.Money fee
}

service Audited {
  Stamp last() throws (1: types.NotFound nf)
}
