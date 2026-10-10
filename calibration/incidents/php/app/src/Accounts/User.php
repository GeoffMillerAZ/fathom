<?php

declare(strict_types=1);

namespace App\Accounts;

/** A user account. */
class User
{
    public function __construct(private string $name, private string $email)
    {
    }

    public function rename(string $full): void
    {
        $parts = explode(' ', $full, 2);
        $this->name = $parts[0] . ' ' . strtoupper($parts[1] ?? throw new \LengthException('no family name'));
    }

    public function domain(): string
    {
        $parts = explode('@', $this->email);
        return strtolower($parts[1] ?? throw new \DomainException('email has no domain'));
    }
}
