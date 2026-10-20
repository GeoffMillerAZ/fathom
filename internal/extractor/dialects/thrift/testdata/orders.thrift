namespace go acme.orders

include "shared.thrift"
include "audit.thrift"
cpp_include "orders_extra.h"

union Payload {
  1: string text,
  2: shared.Money amount
}

struct Order {
  1: required i64 id,
  2: shared.Money total,
  3: list<LineItem> lines,
  4: optional audit.Stamp stamp,
  5: map<string, Payload> extras
}

struct LineItem {
  1: string sku,
  2: i32 qty = 1
}

exception OrderRejected {
  1: string reason,
  2: shared.NotFound cause
}

service Orders extends audit.Audited {
  Order get(1: i64 id) throws (1: shared.NotFound nf),
  list<Order> search(1: string query,
                     2: i32 limit = 10,
                     3: shared.Currency currency)
      throws (1: OrderRejected rejected, 2: shared.NotFound nf),
  oneway void ping();
  void cancel(1: i64 id)
}

service Empty extends Orders {}

service Archive extends Orders {
  void purge()
}
