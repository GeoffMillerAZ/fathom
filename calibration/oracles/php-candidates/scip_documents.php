<?php
declare(strict_types=1);
require $argv[1];
$index = new Scip\Index();
$index->mergeFromString(file_get_contents($argv[2]));
$paths = [];
foreach ($index->getDocuments() as $document) {
    $paths[] = $document->getRelativePath();
}
sort($paths);
echo json_encode($paths, JSON_THROW_ON_ERROR), PHP_EOL;
