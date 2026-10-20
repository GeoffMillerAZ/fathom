include "common/types.thrift"
include "audit.thrift"
cpp_include "common/ignored.thrift"

struct Money {
  1: i32 cents
}

struct Order {
  1: types.Money total,
  2: Money local,
  3: audit.Stamp stamp
}

service Orders extends audit.Audited {
  Order get(1: i64 id) throws (1: types.NotFound nf)
}
