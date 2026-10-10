"""TL-4's log capture runner (docs/plans/telemetry-links.md).

Runs every logging case through the real libraries (the standard logging
module and structlog, installed by ../../../install-log-deps.sh into the
directory INCIDENT_DEPS names), and writes one JSON line per case: the
rendered log line as an operator would paste it, and the truth, the call
site the runtime itself reports (a LogRecord's pathname, lineno and
funcName; structlog's CallsiteParameterAdder). The rendered line never
carries the call site. Every case logs a value carrying "SNTL", which no
localize answer may echo.
"""

import json
import logging
import os
import sys

APP_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, APP_ROOT)
if os.environ.get("INCIDENT_DEPS"):
    sys.path.insert(0, os.environ["INCIDENT_DEPS"])

import structlog  # noqa: E402

from shop.audit import events  # noqa: E402
from shop.fulfillment import dispatch  # noqa: E402
from shop.fulfillment.warehouse import Warehouse  # noqa: E402
from shop.payments import refunds  # noqa: E402
from shop.payments.gateway import Gateway  # noqa: E402

CAPTURED = []


def rel(path):
    return os.path.relpath(path, APP_ROOT).replace(os.sep, "/")


class Capture(logging.Handler):
    """Keeps each record's first rendered line and its call site."""

    def emit(self, record):
        line = self.format(record).split("\n", 1)[0]
        CAPTURED.append(("python.logging", line, {"file": rel(record.pathname), "line": record.lineno, "func": record.funcName, "name": record.funcName}))


def configure_logging():
    handler = Capture()
    handler.setFormatter(logging.Formatter("%(asctime)s %(levelname)s [%(name)s] %(message)s"))
    root = logging.getLogger()
    root.handlers[:] = [handler]
    root.setLevel(logging.DEBUG)


SITE = {}


def take_site(_, __, event_dict):
    SITE["file"] = rel(event_dict.pop("pathname"))
    SITE["line"] = event_dict.pop("lineno")
    SITE["func"] = event_dict.pop("func_name")
    return event_dict


class LineLogger:
    def msg(self, line):
        CAPTURED.append(("python.structlog", line.split("\n", 1)[0], {"file": SITE["file"], "line": SITE["line"], "func": SITE["func"], "name": SITE["func"]}))

    info = debug = warning = error = critical = exception = log = msg


def configure_structlog(renderer):
    structlog.configure(
        processors=[
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso", utc=True),
            structlog.processors.CallsiteParameterAdder([
                structlog.processors.CallsiteParameter.PATHNAME,
                structlog.processors.CallsiteParameter.LINENO,
                structlog.processors.CallsiteParameter.FUNC_NAME,
            ]),
            take_site,
            renderer,
        ],
        wrapper_class=structlog.make_filtering_bound_logger(logging.DEBUG),
        logger_factory=lambda *args: LineLogger(),
        cache_logger_on_first_use=False,
    )


CONSOLE = structlog.dev.ConsoleRenderer(colors=False)
JSON = structlog.processors.JSONRenderer()
KEYVALUE = structlog.processors.KeyValueRenderer(key_order=["timestamp", "level", "event"])

gw = Gateway("SNTL-merchant-01")
wh = Warehouse("SNTL-site-02")

CASES = [
    ("payments-authorize", None, lambda: gw.authorize("4111111111111111", 1250)),
    ("payments-refuse", None, lambda: gw.authorize("SNTL0003", -5)),
    ("payments-capture", None, lambda: gw.capture("SNTL-auth-04", 990)),
    ("payments-decline", None, lambda: gw.decline("SNTL-auth-05", "SNTL05")),
    ("payments-timeout", None, lambda: gw.timeout("SNTL-auth-06", 30)),
    ("payments-void-exception", None, lambda: gw.void("SNTL-auth-07")),
    ("refunds-request", None, lambda: refunds.request_refund("SNTL-pay-08", 400, "damaged")),
    ("refunds-reject-percent-op", None, lambda: refunds.reject_refund("SNTL-pay-09", 50000)),
    ("refunds-settle-log-level", None, lambda: refunds.settle_refund("SNTL-pay-10", "B-7")),
    ("refunds-reverse-concat", None, lambda: refunds.reverse_refund("SNTL-pay-11", "ops-2")),
    ("refunds-flag-dynamic", None, lambda: refunds.flag_refund("SNTL-pay-12")),
    ("warehouse-pick-format", None, lambda: wh.pick("SNTL-sku-13", "A-4")),
    ("warehouse-short-pick-multiline", None, lambda: wh.short_pick("SNTL-sku-14", 6, 2)),
    ("warehouse-restock-implicit-concat", None, lambda: wh.restock("SNTL-sku-15", 40)),
    ("warehouse-quarantine-root", None, lambda: wh.quarantine("SNTL-lot-16")),
    ("warehouse-recount-fstring", None, lambda: wh.recount("SNTL-sku-17")),
    ("dispatch-hand-over", CONSOLE, lambda: dispatch.hand_over("SNTL-parcel-18", "dhl")),
    ("dispatch-missed-pickup", CONSOLE, lambda: dispatch.missed_pickup("SNTL-carrier-19", "09:00-12:00")),
    ("dispatch-label-rejected", CONSOLE, lambda: dispatch.label_rejected("SNTL-parcel-20", "address")),
    ("dispatch-manifest-positional", CONSOLE, lambda: dispatch.manifest_closed("SNTL-manifest-21", 12)),
    ("audit-login-failed-json", JSON, lambda: events.login_failed("SNTL-user-22", 5)),
    ("audit-role-granted-json", JSON, lambda: events.role_granted("SNTL-user-23", "admin", "u-1")),
    ("audit-export-denied-kv", KEYVALUE, lambda: events.export_denied("SNTL-user-24", "orders")),
    ("audit-password-reset-kv", KEYVALUE, lambda: events.password_reset("SNTL-user-25")),
]


def main():
    configure_logging()
    for case_id, renderer, fn in CASES:
        if renderer is not None:
            configure_structlog(renderer)
        del CAPTURED[:]
        fn()
        if not CAPTURED:
            sys.stderr.write("case %s logged nothing\n" % case_id)
            sys.exit(1)
        api, line, truth = CAPTURED[-1]  # the case's own call logs last
        print(json.dumps({"case": case_id, "lang": "python", "api": api, "truth": truth, "payloads": {"log": line}}))


if __name__ == "__main__":
    main()
