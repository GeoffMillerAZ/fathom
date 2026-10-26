<?php
// Transport the fork's own SCIP section, never synthesize reference pairs.
declare(strict_types=1);
require $argv[1];
$raw = json_decode(file_get_contents($argv[2]), true, flags: JSON_THROW_ON_ERROR);
if (!isset($raw['scip']['documents'])) {
    throw new RuntimeException('missing native SCIP section');
}
$index = new Scip\Index();
$index->mergeFromJsonString(json_encode($raw['scip'], JSON_THROW_ON_ERROR));
if (count($index->getDocuments()) !== count($raw['scip']['documents'])) {
    throw new RuntimeException('lost documents during protobuf transport');
}
foreach ($index->getDocuments() as $number => $document) {
    if (count($document->getOccurrences()) !== count($raw['scip']['documents'][$number]['occurrences'])) {
        throw new RuntimeException('lost native occurrences during protobuf transport');
    }
}
file_put_contents($argv[3], $index->serializeToString());
