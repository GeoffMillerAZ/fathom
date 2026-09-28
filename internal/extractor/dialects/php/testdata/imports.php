<?php

namespace App\Consumer;

use Psr\Log\LoggerInterface;
use App\Util\Helper as Assist;
use App\Util\{Str, Arr as A, Deep\Thing};
use function App\Util\format;
use const App\Util\VERSION;
use \App\Legacy\Old;

final class Reader
{
    public function load(string $path): string
    {
        return Assist::load($path) . format($path);
    }
}
