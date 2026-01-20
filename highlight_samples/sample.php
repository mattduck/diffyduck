<?php
/**
 * Sample PHP file for syntax highlighting
 *
 * @package Sample
 * @version 1.0.0
 */

declare(strict_types=1);

namespace App\Sample;

use DateTime;
use Exception;
use InvalidArgumentException;

// Constants
define('APP_NAME', 'Sample Application');
const VERSION = '1.0.0';
const DEBUG = true;

/**
 * User class representing a system user.
 */
class User
{
    private int $id;
    private string $name;
    private string $email;
    private ?DateTime $createdAt;

    public function __construct(int $id, string $name, string $email)
    {
        $this->id = $id;
        $this->name = $name;
        $this->email = $email;
        $this->createdAt = new DateTime();
    }

    public function getId(): int
    {
        return $this->id;
    }

    public function getName(): string
    {
        return $this->name;
    }

    public function setName(string $name): void
    {
        if (empty($name)) {
            throw new InvalidArgumentException('Name cannot be empty');
        }
        $this->name = $name;
    }

    public function getEmail(): string
    {
        return $this->email;
    }

    public function toArray(): array
    {
        return [
            'id' => $this->id,
            'name' => $this->name,
            'email' => $this->email,
            'created_at' => $this->createdAt?->format('Y-m-d H:i:s'),
        ];
    }
}

/**
 * Interface for user repositories.
 */
interface UserRepositoryInterface
{
    public function find(int $id): ?User;
    public function save(User $user): void;
    public function delete(int $id): bool;
}

/**
 * Trait for logging functionality.
 */
trait LoggerTrait
{
    protected function log(string $message, string $level = 'info'): void
    {
        $timestamp = date('Y-m-d H:i:s');
        echo "[{$timestamp}] [{$level}] {$message}\n";
    }
}

/**
 * User service class.
 */
class UserService implements UserRepositoryInterface
{
    use LoggerTrait;

    private array $users = [];

    public function find(int $id): ?User
    {
        return $this->users[$id] ?? null;
    }

    public function save(User $user): void
    {
        $this->users[$user->getId()] = $user;
        $this->log("User {$user->getName()} saved", 'info');
    }

    public function delete(int $id): bool
    {
        if (isset($this->users[$id])) {
            unset($this->users[$id]);
            $this->log("User {$id} deleted", 'warning');
            return true;
        }
        return false;
    }

    public function findAll(): array
    {
        return array_values($this->users);
    }
}

// Example usage
$service = new UserService();

$users = [
    new User(1, 'Alice', 'alice@example.com'),
    new User(2, 'Bob', 'bob@example.com'),
    new User(3, 'Charlie', 'charlie@example.com'),
];

foreach ($users as $user) {
    $service->save($user);
}

// Array functions
$names = array_map(fn(User $u) => $u->getName(), $service->findAll());
$filtered = array_filter($users, fn(User $u) => $u->getId() > 1);

// String interpolation
$greeting = "Hello, {$users[0]->getName()}!";
$heredoc = <<<EOT
This is a heredoc string.
User: {$users[0]->getName()}
Email: {$users[0]->getEmail()}
EOT;

// Match expression (PHP 8+)
$status = match ($users[0]->getId()) {
    1 => 'first',
    2 => 'second',
    default => 'other',
};

// Null coalescing
$name = $_GET['name'] ?? 'Guest';
$config = $settings['debug'] ?? false;

// Spread operator
$allUsers = [...$users, new User(4, 'Diana', 'diana@example.com')];

// Arrow functions
$doubled = array_map(fn($x) => $x * 2, [1, 2, 3, 4, 5]);

echo json_encode($service->findAll()[0]->toArray(), JSON_PRETTY_PRINT);
