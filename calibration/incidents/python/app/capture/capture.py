"""TL-7's capture runner (docs/plans/telemetry-links.md).

Runs every fault-injected case, reads the truth from Python's own
structured traceback (the root cause's innermost frame inside the app),
and writes one JSON line per case carrying the raw traceback and the
payload shapes a notifier, a tracer, the Lambda runtime and an APM agent
would send. The app root is rewritten to /app (a container's deploy root;
/var/task for Lambda).
"""

import json
import os
import sys
import traceback

APP_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, APP_ROOT)

from shop.accounts.auth import Auth  # noqa: E402
from shop.accounts.user import User  # noqa: E402
from shop.billing.invoice import Invoice  # noqa: E402
from shop.catalog.product import Product  # noqa: E402
from shop.catalog.search import Search  # noqa: E402
from shop.orders.cart import Cart  # noqa: E402
from shop.orders.order import Order  # noqa: E402
from shop.shipping.shipment import Shipment  # noqa: E402


def _summarize():
    o = Order(5)
    o.add_line(3)
    o.summarize(0)


CASES = [
    ("billing-total-div0", lambda: Invoice([100, 200], "USD").total(0)),
    ("billing-tax-none", lambda: Invoice([100], "USD").apply_tax("US")),
    ("billing-parse-amount", lambda: Invoice([1], "USD").parse_and_add("12a")),
    ("billing-ctor-currency", lambda: Invoice([], "EURO")),
    ("billing-refund-static", lambda: Invoice.refund(Invoice([10], "USD"), 9999)),
    ("shipping-round-div0", lambda: Shipment(1200, "DE").packed(0)),
    ("shipping-band-bounds", lambda: Shipment(5000, "DE").band()),
    ("shipping-validate", lambda: Shipment(10, "").validate()),
    ("shipping-track-none", lambda: Shipment(10, "FR").track({})),
    ("catalog-validate", lambda: Product("bad", "1").validate()),
    ("catalog-walk-cycle", lambda: Search({"a": "b", "b": "a"}).depth("a")),
    ("catalog-rank-missing", lambda: Search({}).rank(["A"], "Z")),
    ("catalog-price-parse", lambda: Product("ABC-1", "x").price()),
    ("orders-add-line", lambda: Order(5).add_line(0)),
    ("orders-coupon-none", lambda: Cart(Order(5), {}).apply_coupon("NOPE")),
    ("orders-checkout-chained", lambda: Cart(Order(5), {}).checkout(99)),
    ("orders-summarize-nested", _summarize),
    ("orders-deep-chain", lambda: Order(1).deep(5)),
    ("accounts-rename", lambda: User("a", "a@b.c").rename("Single")),
    ("accounts-verify", lambda: Auth("s3cret").verify("token")),
    ("accounts-hash-algorithm", lambda: Auth("s").hash_password("p", "NOPE-256")),
    ("accounts-domain", lambda: User("a", "nope").domain()),
]


def in_app(path):
    return path.startswith(APP_ROOT + os.sep) and not path.startswith(os.path.join(APP_ROOT, "capture") + os.sep)


def rel(path):
    return os.path.relpath(path, APP_ROOT).replace(os.sep, "/")


def deployed(text, root):
    return text.replace(APP_ROOT + os.sep, root + "/")


def chain(e):
    out = []
    while e is not None:
        out.append(e)
        e = e.__cause__ or (None if e.__suppress_context__ else e.__context__)
    return out


def attr(key, v):
    return {"key": key, "value": {"intValue": str(v)} if isinstance(v, int) else {"stringValue": str(v)}}


def record(case_id, e):
    # The deepest error in the chain whose trace reaches the app: a library
    # error raised under an app function belongs to that function.
    truth = None
    for x in reversed(chain(e)):
        frames = [f for f in traceback.extract_tb(x.__traceback__) if in_app(f.filename)]
        if frames:
            truth = frames[-1]
            break
    raw = deployed("".join(traceback.format_exception(e)), "/app")
    exceptions = []
    for x in chain(e):
        st = []
        for f in reversed(traceback.extract_tb(x.__traceback__)):
            st.append({"file": rel(f.filename) if f.filename.startswith(APP_ROOT) else f.filename, "lineNumber": f.lineno, "method": f.name, "inProject": f.filename.startswith(APP_ROOT)})
        exceptions.append({"errorClass": type(x).__name__, "message": str(x), "stacktrace": st})
    bugsnag = {
        "apiKey": "0" * 32,
        "events": [{
            "exceptions": exceptions,
            "breadcrumbs": [{"name": "checkout clicked", "type": "user"}],
            "user": {"id": "u-17"},
            "metaData": {"request": {"path": "/checkout"}},
        }],
    }
    here = traceback.extract_stack(limit=1)[0]
    otlp = {"resourceLogs": [{
        "resource": {"attributes": [attr("service.name", "shop-python")]},
        "scopeLogs": [{"logRecords": [{
            "severityText": "ERROR",
            "body": {"stringValue": str(e)},
            "attributes": [
                attr("exception.type", type(e).__name__),
                attr("exception.message", str(e)),
                attr("exception.stacktrace", raw),
                attr("code.filepath", deployed(here.filename, "/app")),
                attr("code.lineno", here.lineno),
                attr("code.function", here.name),
            ],
        }]}],
    }]}
    lam = {
        "errorMessage": str(e),
        "errorType": type(e).__name__,
        "requestId": "00000000-0000-0000-0000-000000000000",
        "stackTrace": [deployed(s, "/var/task") for s in traceback.format_list(traceback.extract_tb(e.__traceback__))],
    }
    newrelic = {"results": [{"events": [{"error.class": type(e).__name__, "error.message": str(e), "error.stack_trace": raw, "appName": "shop-python"}]}]}
    return json.dumps({
        "case": case_id,
        "lang": "python",
        "truth": {"file": rel(truth.filename), "line": truth.lineno, "func": truth.name, "name": truth.name},
        "payloads": {
            "raw": raw,
            "bugsnag": deployed(json.dumps(bugsnag), "/app"),
            "otlp": json.dumps(otlp),
            "lambda": json.dumps(lam),
            "newrelic": json.dumps(newrelic),
        },
    })


def main():
    for case_id, fn in CASES:
        try:
            fn()
        except Exception as e:  # noqa: BLE001 - every case must throw
            print(record(case_id, e))
            continue
        sys.stderr.write("case %s did not throw\n" % case_id)
        sys.exit(1)


if __name__ == "__main__":
    main()
