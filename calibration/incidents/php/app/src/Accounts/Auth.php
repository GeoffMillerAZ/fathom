<?php

declare(strict_types=1);

namespace App\Accounts;

/** Authentication: tokens and password hashing. */
class Auth
{
    public function __construct(private string $secret)
    {
    }

    public function verify(string $token): void
    {
        if (!str_ends_with($token, $this->secret)) {
            throw new \RuntimeException('token rejected');
        }
    }

    public function hashPassword(string $password, string $algorithm): string
    {
        return hash($algorithm, $password);
    }
}
