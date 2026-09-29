<?php
namespace Alpha\Core {
    interface Store
    {
        public function get(string $key): ?string;
    }

    class MemoryStore implements Store
    {
        private array $data = [];

        public function get(string $key): ?string
        {
            return $this->data[$key] ?? null;
        }
    }
}

namespace Beta {
    use Alpha\Core\Store;

    function connect(Store $s): void
    {
        $s->get('x');
    }

    trait Cached
    {
        public function warm(): void
        {
            $this->flush();
        }
    }
}

namespace {
    function global_helper(): int
    {
        return 1;
    }
}
