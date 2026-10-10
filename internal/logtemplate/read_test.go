package logtemplate

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// render prints a fact as "L<line> <api> <text> [hole spellings]" for the
// table tests: an integer hole is marked #.
func render(f Fact) string {
	var hs []string
	for _, h := range f.Holes {
		s := f.Text[h.Start:h.End]
		if h.Int {
			s += "#"
		}
		hs = append(hs, s)
	}
	out := fmt.Sprintf("L%d %s %q %v", f.Line, f.API, f.Text, hs)
	if len(f.Keys) > 0 {
		out += " keys=" + strings.Join(f.Keys, ",")
	}
	return out
}

func readAll(path, src string) []string {
	var out []string
	for _, f := range Read(path, []byte(src)) {
		out = append(out, render(f))
	}
	return out
}

// TestReadPerLogger is the reader's table: one source per logger API, every
// supported message shape, and the shapes that are never a template (a
// call in a comment or a string, a message with no literal text, an
// unknown receiver).
func TestReadPerLogger(t *testing.T) {
	cases := []struct {
		name, path, src string
		want            []string
	}{
		{
			name: "python logging", path: "shop/billing/invoice.py",
			src: `import logging
logger = logging.getLogger(__name__)

class Invoice:
    def total(self, n):
        logger.info("invoice %s total %d", self.id, n)
        logger.warning("tax %(region)s missing", {"region": self.region})
        logger.error(f"order {self.oid} failed")
        logger.exception("refund failed for %s", rid)
        logging.info("100% done")
        logger.log(logging.ERROR, "level %s here", n)
        logger.info("first part "
                    "second %s", n)
        logger.info("total: " + str(n) + " cents")
        logger.info("user {} logged in".format(n))
        logger.info("rate %.2f%% ok" % n)
        logger.debug("%s", n)
        # logger.info("in a comment %s", n)
        s = "logger.info('in a string %s', n)"
        other.info("unknown receiver %s", n)
        logger.info("kw only %s", extra={"a": 1})
`,
			want: []string{
				`L6 python.logging "invoice %s total %d" [%s %d#]`,
				`L7 python.logging "tax %(region)s missing" [%(region)s]`,
				`L8 python.logging "order {self.oid} failed" [{self.oid}]`,
				`L9 python.logging "refund failed for %s" [%s]`,
				`L10 python.logging "100% done" []`,
				`L11 python.logging "level %s here" [%s]`,
				`L12 python.logging "first part second %s" [%s]`,
				`L14 python.logging "total: {str(n)} cents" [{str(n)}]`,
				`L15 python.logging "user {} logged in" [{}]`,
				`L16 python.logging "rate %.2f% ok" [%.2f]`,
				`L21 python.logging "kw only %s" []`,
			},
		},
		{
			name: "python structlog", path: "shop/orders/cart.py",
			src: `import structlog
log = structlog.get_logger()

def checkout(order_id, reason):
    log.info("payment_failed", order_id=order_id, reason=reason, exc_info=True)
    log.warning("coupon %s rejected", code)
    structlog.get_logger().error("stock_low", sku=sku)
`,
			want: []string{
				`L5 python.structlog "payment_failed" [] keys=order_id,reason`,
				`L6 python.structlog "coupon %s rejected" [%s]`,
				`L7 python.structlog "stock_low" [] keys=sku`,
			},
		},
		{
			name: "java slf4j", path: "src/main/java/com/acme/Invoice.java",
			src: `package com.acme;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
class Invoice {
  private static final Logger log = LoggerFactory.getLogger(Invoice.class);
  void total(int n) {
    log.info("invoice {} total {}", id, n);
    log.warn("no args {}");
    log.error("order " + id + " failed", e);
    log.info(String.format("rate %.2f%% for %s and %d", r, c, n));
    log.debug("escaped \\{} literal {}", x);
    log.info(MARKER, "marked {}", x);
    // log.info("commented {}", x);
  }
}
`,
			want: []string{
				`L7 java.slf4j "invoice {} total {}" [{} {}]`,
				`L8 java.slf4j "no args {}" []`,
				`L9 java.slf4j "order {id} failed" [{id}]`,
				`L10 java.slf4j "rate %.2f% for %s and %d" [%.2f %s %d#]`,
				`L11 java.slf4j "escaped {} literal {}" [{}]`,
				`L12 java.slf4j "marked {}" [{}]`,
			},
		},
		{
			name: "java log4j2", path: "src/main/java/com/acme/Charge.java",
			src: `package com.acme;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;
class Charge {
  private static final Logger LOG = LogManager.getLogger(Charge.class);
  void run() {
    LOG.error("charge {} declined", id);
    LOG.log(Level.WARN, "retry {} of {}", n, max);
    LOG.fatal("gateway down");
  }
}
`,
			want: []string{
				`L7 java.log4j2 "charge {} declined" [{}]`,
				`L8 java.log4j2 "retry {} of {}" [{} {}]`,
				`L9 java.log4j2 "gateway down" []`,
			},
		},
		{
			name: "java jul", path: "src/main/java/com/acme/Ship.java",
			src: `package com.acme;
import java.util.logging.Level;
import java.util.logging.Logger;
class Ship {
  private static final Logger LOGGER = Logger.getLogger(Ship.class.getName());
  void send() {
    LOGGER.info("plain shipment message");
    LOGGER.log(Level.WARNING, "carrier {0} rejected {1}", new Object[]{a, b});
    LOGGER.log(Level.INFO, "it''s parcel {0}", id);
    LOGGER.severe("label {0} kept");
  }
}
`,
			want: []string{
				`L7 java.jul "plain shipment message" []`,
				`L8 java.jul "carrier {0} rejected {1}" [{0} {1}]`,
				`L9 java.jul "it's parcel {0}" [{0}]`,
				`L10 java.jul "label {0} kept" []`,
			},
		},
		{
			name: "node pino", path: "src/billing/charge.js",
			src: `const pino = require('pino');
const logger = pino();
function charge(id, n) {
  logger.info({ orderId: id }, 'charge %s declined', id);
  logger.error(` + "`order ${id} failed`" + `);
  console.error('cart %d items, %i whole', n, n);
  logger.warn('plain ' + id + ' done');
  logger.info(util.format('%s of %s', a, b));
}
`,
			want: []string{
				`L4 node.pino "charge %s declined" [%s]`,
				`L5 node.pino "order ${id} failed" [${id}]`,
				`L6 node.console "cart %d items, %i whole" [%d %i#]`,
				`L7 node.pino "plain {id} done" [{id}]`,
			},
		},
		{
			name: "node winston", path: "src/ship/label.ts",
			src: `import winston from 'winston';
const log: winston.Logger = winston.createLogger({});
export function label(d: string) {
  log.log('error', 'disk %s full', d);
  log.info(` + "`user ${u.name} logged in`" + `);
  log.warn('quota', { user: u });
}
`,
			want: []string{
				`L4 node.winston "disk %s full" [%s]`,
				`L5 node.winston "user ${u.name} logged in" [${u.name}]`,
				`L6 node.winston "quota" []`,
			},
		},
		{
			name: "php psr3 monolog", path: "src/Billing/Invoice.php",
			src: `<?php
namespace App\Billing;
use Psr\Log\LoggerInterface;
class Invoice {
    public function total() {
        $this->logger->error('order {id} failed', ['id' => $id]);
        $this->logger->warning(sprintf('rate %.2f%% for %s', $r, $c));
        $this->logger->info("user $name logged {$when} in");
        $this->logger->info('total ' . $t . ' cents');
        $this->logger->log(LogLevel::ALERT, 'disk {disk.name} full', ['disk.name' => $d]);
        # $this->logger->info('comment {x}');
        Log::notice('facade {x} call', ['x' => 1]);
    }
}
`,
			want: []string{
				`L6 php.psr3 "order {id} failed" [{id}]`,
				`L7 php.psr3 "rate %.2f% for %s" [%.2f %s]`,
				`L8 php.psr3 "user $name logged {$when} in" [$name {$when}]`,
				`L9 php.psr3 "total {$t} cents" [{$t}]`,
				`L10 php.psr3 "disk {disk.name} full" [{disk.name}]`,
				`L12 php.psr3 "facade {x} call" [{x}]`,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := readAll(c.path, c.src)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Read:\n got  %s\n want %s", strings.Join(got, "\n      "), strings.Join(c.want, "\n      "))
			}
		})
	}
}

// TestReadIgnoresUnreadFiles: a file the registry names no language for
// yields nothing, and Reads says so.
func TestReadIgnoresUnreadFiles(t *testing.T) {
	if got := Read("main.go", []byte(`log.Printf("x %s", y)`)); got != nil {
		t.Errorf("Read(main.go) = %v, want nil", got)
	}
	if Reads("main.go") || !Reads("a/b.py") || !Reads("x.tsx") || !Reads("Y.java") || !Reads("z.php") {
		t.Error("Reads: wrong candidate set")
	}
}

// TestReadIsDeterministic: the same bytes read twice give the same facts.
func TestReadIsDeterministic(t *testing.T) {
	src := "import logging\nlog = logging.getLogger()\ndef f():\n    log.info('one %s', a)\n    log.info('two %s', b)\n"
	a, b := Read("x.py", []byte(src)), Read("x.py", []byte(src))
	if !reflect.DeepEqual(a, b) || len(a) != 2 {
		t.Errorf("Read not deterministic or wrong count: %v vs %v", a, b)
	}
}

// TestReadNeverPanicsOnTruncatedSource: a file cut anywhere (an
// unterminated string, an unbalanced call, half a receiver) reads without
// a panic; the reader yields what it can and guesses nothing.
func TestReadNeverPanicsOnTruncatedSource(t *testing.T) {
	srcs := map[string]string{
		"a.py":   "import structlog\nlog = structlog.get_logger()\ndef f():\n    log.info(f\"x {a['k']} y\", k=1)\n    logging.getLogger(__name__).warning('b %s', (c))\n",
		"A.java": "class A { void f() { log.info(\"a {} \\\"q\\\"\", x); LOG.log(Level.INFO, \"\"\"\n  text {}\n  \"\"\", y); } }\n",
		"a.ts":   "const log = pino();\nlog.info({a: `t ${b + `n ${c}`}`}, 'm %s', d);\nconsole.error(`x ${y}`);\n",
		"a.php":  "<?php $this->logger->info(\"u {$a['b']} $c->d e\", ['x' => 1]); self::$logger->log('w', <<<EOT\nhi\nEOT);\n",
	}
	for path, src := range srcs {
		for i := 0; i <= len(src); i++ {
			_ = Read(path, []byte(src[:i]))
		}
	}
}
