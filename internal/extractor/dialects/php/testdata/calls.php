<?php
namespace App\Calls;

use App\Net\Service;

function plain(): void
{
    strlen('a');
    \strtoupper('a');
    \App\Util\qualified('a');
    namespace\local_fn();
    $callable = 'plain';
    $callable();
}

class Caller extends Base
{
    public function run(Service $svc): void
    {
        $this->helper();
        $svc->fetch()->decode();
        $svc?->maybe();
        Service::create();
        static::boot();
        parent::run($svc);
        $made = new Service();
        (new Service())->fluent();
        array_map(fn($x) => $this->wrap($x), [1, 2]);
    }

    private function helper(
        int $one,
        int $two
    ): int {
        return $this->sum(
            $one,
            $two
        );
    }
}
