<?php

declare(strict_types=1);

// TL-4's log capture runner (docs/plans/telemetry-links.md): runs every
// logging case through Monolog (installed by ../../install-log-deps.sh
// into the directory INCIDENT_DEPS names) with its PSR-3 message processor,
// and writes one JSON line per case: the rendered log line as an operator
// would paste it (Monolog's default line format), and the truth, the call
// site Monolog's own IntrospectionProcessor reads from PHP's backtrace. The
// rendered line never carries the call site. Every case logs a value
// carrying "SNTL", which no localize answer may echo.

use App\Audit\Events;
use App\Fulfillment\Warehouse;
use App\Payments\Gateway;
use App\Payments\Refunds;
use Monolog\Formatter\LineFormatter;
use Monolog\Handler\AbstractProcessingHandler;
use Monolog\Level;
use Monolog\Logger;
use Monolog\LogRecord;
use Monolog\Processor\IntrospectionProcessor;
use Monolog\Processor\PsrLogMessageProcessor;

$appRoot = dirname(__DIR__);
require getenv('INCIDENT_DEPS') . '/vendor/autoload.php';

spl_autoload_register(function (string $class) use ($appRoot): void {
    if (str_starts_with($class, 'App\\')) {
        require $appRoot . '/src/' . str_replace('\\', '/', substr($class, 4)) . '.php';
    }
});

final class CaptureHandler extends AbstractProcessingHandler
{
    /** @var list<array<string, mixed>> */
    public array $captured = [];

    public function __construct(private string $appRoot)
    {
        parent::__construct(Level::Debug);
        $this->setFormatter(new LineFormatter());
    }

    protected function write(LogRecord $record): void
    {
        $x = $record->extra;
        $line = (string) $this->getFormatter()->format($record->with(extra: []));
        $this->captured[] = [
            'line' => explode("\n", rtrim($line, "\n"))[0],
            'truth' => [
                'file' => substr($x['file'], strlen($this->appRoot) + 1),
                'line' => $x['line'],
                'func' => $x['function'],
                'name' => $x['function'],
            ],
        ];
    }
}

$handler = new CaptureHandler($appRoot);
$logger = new Logger('shop', [$handler], [new PsrLogMessageProcessor(), new IntrospectionProcessor(Level::Debug)]);
Events::$logger = $logger;
$gw = new Gateway($logger, 'SNTL-merchant-01');
$refunds = new Refunds($logger);
$wh = new Warehouse($logger, 'SNTL-site-02');

$cases = [
    'payments-authorize' => fn () => $gw->authorize(1250, '4111111111111111'),
    'payments-refuse' => fn () => $gw->authorize(-5, 'SNTL0003'),
    'payments-decline-sprintf' => fn () => $gw->decline('SNTL-auth-04', 'SNTL04'),
    'payments-timeout-interpolation' => fn () => $gw->timeout('SNTL-auth-05', 30),
    'payments-void-exception-context' => fn () => $gw->voidAuthorization('SNTL-auth-06'),
    'payments-capture-concat' => fn () => $gw->capture('SNTL-auth-07', 990),
    'payments-chargeback' => fn () => $gw->chargeback('SNTL-pay-08', 'fraud'),
    'refunds-request' => fn () => $refunds->request('SNTL-pay-09', 400, 'damaged'),
    'refunds-reject-sprintf' => fn () => $refunds->reject('SNTL-pay-10', 50000),
    'refunds-settle-debug' => fn () => $refunds->settle('SNTL-pay-11', 'B-7'),
    'refunds-flag-dynamic' => fn () => $refunds->flag('SNTL-pay-12'),
    'refunds-credit-log-level' => fn () => $refunds->credit('SNTL-refund-13', 400),
    'warehouse-pick' => fn () => $wh->pick('SNTL-sku-14', 'A-4'),
    'warehouse-short-pick' => fn () => $wh->shortPick('SNTL-sku-15', 6, 2),
    'warehouse-restock-multiline' => fn () => $wh->restock('SNTL-sku-16', 40),
    'warehouse-quarantine-log-level' => fn () => $wh->quarantine('SNTL-lot-17'),
    'warehouse-recount-interpolation' => fn () => $wh->recount('SNTL-sku-18'),
    'warehouse-cycle-count' => fn () => $wh->cycleCountFailed('SNTL-sku-19', 3),
    'audit-login-failed-static' => fn () => Events::loginFailed('SNTL-user-20', 5),
    'audit-role-granted' => fn () => Events::roleGranted('SNTL-user-21', 'admin', 'u-1'),
    'audit-export-denied' => fn () => Events::exportDenied('SNTL-user-22', 'orders'),
    'audit-password-reset-concat' => fn () => Events::passwordReset('SNTL-user-23'),
    'audit-access-denied-interpolation' => fn () => Events::accessDenied('SNTL-user-24', 'reports'),
    'audit-session-expired-dotted' => fn () => Events::sessionExpired('SNTL-session-25'),
];

$out = '';
foreach ($cases as $id => $fn) {
    $handler->captured = [];
    $fn();
    if ($handler->captured === []) {
        fwrite(STDERR, "case $id logged nothing\n");
        exit(1);
    }
    $last = $handler->captured[count($handler->captured) - 1];
    $out .= json_encode(['case' => $id, 'lang' => 'php', 'api' => 'php.psr3', 'truth' => $last['truth'], 'payloads' => ['log' => $last['line']]], JSON_UNESCAPED_SLASHES) . "\n";
}
echo $out;
