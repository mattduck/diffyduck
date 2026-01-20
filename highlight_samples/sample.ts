// Sample TypeScript for syntax highlighting

'use strict';

// Type definitions
type ID = string | number;
type Status = 'active' | 'inactive' | 'pending';
type Callback<T> = (data: T) => void;

// Interface
interface User {
    readonly id: ID;
    name: string;
    email: string;
    status: Status;
    metadata?: Record<string, unknown>;
}

interface ApiResponse<T> {
    data: T;
    error?: string;
    timestamp: Date;
}

// Enum
enum LogLevel {
    DEBUG = 'debug',
    INFO = 'info',
    WARN = 'warn',
    ERROR = 'error',
}

// Constants
const API_URL = 'https://api.example.com' as const;
const DEFAULT_TIMEOUT = 5000;

// Generic class
class Repository<T extends { id: ID }> {
    private items: Map<ID, T> = new Map();

    constructor(private readonly name: string) {}

    add(item: T): void {
        this.items.set(item.id, item);
    }

    get(id: ID): T | undefined {
        return this.items.get(id);
    }

    getAll(): T[] {
        return Array.from(this.items.values());
    }

    delete(id: ID): boolean {
        return this.items.delete(id);
    }

    find(predicate: (item: T) => boolean): T | undefined {
        return this.getAll().find(predicate);
    }
}

// Class with decorators (conceptual)
class UserService {
    private repository: Repository<User>;
    private logger: (msg: string) => void;

    constructor() {
        this.repository = new Repository<User>('users');
        this.logger = console.log;
    }

    async createUser(data: Omit<User, 'id'>): Promise<User> {
        const user: User = {
            id: crypto.randomUUID(),
            ...data,
        };

        this.repository.add(user);
        this.logger(`Created user: ${user.name}`);

        return user;
    }

    getUserById(id: ID): User | undefined {
        return this.repository.get(id);
    }

    findByStatus(status: Status): User[] {
        return this.repository
            .getAll()
            .filter((user) => user.status === status);
    }
}

// Utility types
type Partial<T> = { [P in keyof T]?: T[P] };
type Required<T> = { [P in keyof T]-?: T[P] };
type Pick<T, K extends keyof T> = { [P in K]: T[P] };

// Type guards
function isUser(obj: unknown): obj is User {
    return (
        typeof obj === 'object' &&
        obj !== null &&
        'id' in obj &&
        'name' in obj &&
        'email' in obj
    );
}

// Generic function with constraints
function merge<T extends object, U extends object>(obj1: T, obj2: U): T & U {
    return { ...obj1, ...obj2 };
}

// Async/await with generics
async function fetchData<T>(url: string): Promise<ApiResponse<T>> {
    try {
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data: T = await response.json();
        return {
            data,
            timestamp: new Date(),
        };
    } catch (error) {
        return {
            data: null as unknown as T,
            error: error instanceof Error ? error.message : 'Unknown error',
            timestamp: new Date(),
        };
    }
}

// Arrow functions with types
const double = (n: number): number => n * 2;
const greet = (name: string): string => `Hello, ${name}!`;

const processUsers = (users: User[]): string[] =>
    users
        .filter((u) => u.status === 'active')
        .map((u) => u.name);

// Template literal types
type EventName = `on${Capitalize<string>}`;
type CSSUnit = `${number}${'px' | 'em' | 'rem' | '%'}`;

// Conditional types
type NonNullable<T> = T extends null | undefined ? never : T;
type ReturnType<T> = T extends (...args: unknown[]) => infer R ? R : never;

// Main execution
async function main(): Promise<void> {
    const service = new UserService();

    const users = await Promise.all([
        service.createUser({
            name: 'Alice',
            email: 'alice@example.com',
            status: 'active',
        }),
        service.createUser({
            name: 'Bob',
            email: 'bob@example.com',
            status: 'pending',
        }),
    ]);

    console.log('Created users:', users);

    const activeUsers = service.findByStatus('active');
    console.log('Active users:', activeUsers);

    // Destructuring
    const [first, second] = users;
    const { name, email } = first;

    // Optional chaining and nullish coalescing
    const metadata = first?.metadata ?? {};
    const displayName = first?.name ?? 'Unknown';

    console.log(`${displayName}: ${email}`);
}

main().catch(console.error);

export { User, UserService, Repository, fetchData };
export type { ApiResponse, Status, ID };
