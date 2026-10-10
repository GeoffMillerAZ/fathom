<?php

declare(strict_types=1);

namespace App\Audit;

use Psr\Log\LoggerInterface;

/** Audit events, through one shared logger. */
final class Events
{
    public static LoggerInterface $logger;

    public static function loginFailed(string $user, int $attempts): void
    {
        self::$logger->warning('login failed for user {user} after {attempts} attempts', ['user' => $user, 'attempts' => $attempts]);
    }

    public static function roleGranted(string $user, string $role, string $grantedBy): void
    {
        self::$logger->info('role {role} granted to {user} by {by}', ['role' => $role, 'user' => $user, 'by' => $grantedBy]);
    }

    public static function exportDenied(string $user, string $dataset): void
    {
        self::$logger->alert('export of {dataset} denied for {user}', ['dataset' => $dataset, 'user' => $user]);
    }

    public static function passwordReset(string $user): void
    {
        self::$logger->info('password reset requested for ' . $user);
    }

    public static function accessDenied(string $user, string $resource): void
    {
        self::$logger->error("user {$user} cannot access {$resource}");
    }

    public static function sessionExpired(string $session): void
    {
        self::$logger->notice('session {session.id} expired after inactivity', ['session.id' => $session]);
    }
}
