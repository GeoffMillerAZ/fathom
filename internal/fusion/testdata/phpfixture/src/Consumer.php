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
        return Util\format('x');
    }
}
