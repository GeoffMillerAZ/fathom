<?php

namespace App;

use App\Util;
use App\Util\Helper;
use App\Util\Helper as Assist;

class Consumer
{
    public function run(): string
    {
        Helper::assist();
        self::local();
        $this->local();
        return Util\format('x');
    }

    public function viaAlias(): void
    {
        Assist::assist();
    }

    private static function local(): void
    {
    }
}
