// Shared types for the fixtures.
namespace java com.acme.shared
namespace py acme.shared

typedef i64 Timestamp
typedef map<string, list<Money>> Ledger (cpp.type = "std::map")

const i32 MAX_ITEMS = 100
const string DEFAULT_CURRENCY = "USD"

enum Currency {
  USD = 1,
  EUR,
  GBP = 10 (label = "pound")
}

struct Money {
  1: required i64 units,
  2: optional Currency currency = Currency.USD
}

exception NotFound {
  1: string message
}
