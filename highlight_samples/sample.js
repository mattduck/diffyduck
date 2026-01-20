// Sample JavaScript for syntax highlighting

'use strict';

// Constants and variables
const API_URL = 'https://api.example.com';
let currentUser = null;
var legacyVar = 'deprecated';

// Class definition
class User {
    #privateField = 'secret';

    constructor(name, email) {
        this.name = name;
        this.email = email;
        this.createdAt = new Date();
    }

    static fromJSON(json) {
        return new User(json.name, json.email);
    }

    get displayName() {
        return `${this.name} <${this.email}>`;
    }

    set displayName(value) {
        const [name, email] = value.split(' <');
        this.name = name;
        this.email = email.replace('>', '');
    }

    async fetchProfile() {
        const response = await fetch(`${API_URL}/users/${this.email}`);
        return response.json();
    }

    toString() {
        return this.displayName;
    }
}

// Arrow functions
const double = (x) => x * 2;
const add = (a, b) => a + b;
const greet = name => `Hello, ${name}!`;

// Template literals
const multiline = `
    This is a multiline
    template literal with ${1 + 1} interpolation
`;

// Destructuring
const { name, email } = { name: 'Alice', email: 'alice@example.com' };
const [first, second, ...rest] = [1, 2, 3, 4, 5];

// Spread operator
const merged = { ...{ a: 1 }, ...{ b: 2 } };
const combined = [...[1, 2], ...[3, 4]];

// Async/await
async function fetchData(url) {
    try {
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        return data;
    } catch (error) {
        console.error('Fetch failed:', error);
        throw error;
    }
}

// Promises
const promise = new Promise((resolve, reject) => {
    setTimeout(() => {
        resolve('Done!');
    }, 1000);
});

promise.then(result => console.log(result)).catch(err => console.error(err));

// Higher-order functions
const numbers = [1, 2, 3, 4, 5];
const doubled = numbers.map(n => n * 2);
const evens = numbers.filter(n => n % 2 === 0);
const sum = numbers.reduce((acc, n) => acc + n, 0);

// Regular expressions
const pattern = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/;
const isEmail = pattern.test(email);

// Object methods
const obj = {
    method() {
        return this;
    },
    *generator() {
        yield 1;
        yield 2;
        yield 3;
    },
    async asyncMethod() {
        return await Promise.resolve(42);
    }
};

// Symbols and iterators
const sym = Symbol('description');
const iterable = {
    [Symbol.iterator]() {
        let i = 0;
        return {
            next() {
                return i < 3 ? { value: i++, done: false } : { done: true };
            }
        };
    }
};

// Export
export { User, fetchData };
export default User;
