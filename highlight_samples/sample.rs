//! Sample Rust code for syntax highlighting

use std::collections::HashMap;
use std::fmt::{self, Display, Formatter};
use std::io::{self, Read, Write};
use std::sync::{Arc, Mutex};

/// Maximum number of retries for operations
const MAX_RETRIES: u32 = 3;
static GLOBAL_COUNTER: std::sync::atomic::AtomicUsize = std::sync::atomic::AtomicUsize::new(0);

/// User status enum
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Status {
    Active,
    Inactive,
    Pending,
}

/// A user in the system
#[derive(Debug, Clone)]
pub struct User {
    id: u64,
    name: String,
    email: String,
    status: Status,
    metadata: HashMap<String, String>,
}

impl User {
    /// Creates a new user
    pub fn new(id: u64, name: impl Into<String>, email: impl Into<String>) -> Self {
        Self {
            id,
            name: name.into(),
            email: email.into(),
            status: Status::Active,
            metadata: HashMap::new(),
        }
    }

    /// Returns the user's display name
    pub fn display_name(&self) -> &str {
        &self.name
    }

    /// Sets metadata for the user
    pub fn set_metadata(&mut self, key: impl Into<String>, value: impl Into<String>) {
        self.metadata.insert(key.into(), value.into());
    }
}

impl Display for User {
    fn fmt(&self, f: &mut Formatter<'_>) -> fmt::Result {
        write!(f, "{} <{}>", self.name, self.email)
    }
}

/// Result type for user operations
pub type UserResult<T> = Result<T, UserError>;

/// Error type for user operations
#[derive(Debug)]
pub enum UserError {
    NotFound(u64),
    InvalidInput(String),
    IoError(io::Error),
}

impl From<io::Error> for UserError {
    fn from(err: io::Error) -> Self {
        UserError::IoError(err)
    }
}

/// Trait for user storage
pub trait UserStore: Send + Sync {
    fn get(&self, id: u64) -> UserResult<User>;
    fn save(&mut self, user: User) -> UserResult<()>;
    fn delete(&mut self, id: u64) -> UserResult<bool>;
}

/// In-memory user store implementation
pub struct MemoryStore {
    users: Arc<Mutex<HashMap<u64, User>>>,
}

impl MemoryStore {
    pub fn new() -> Self {
        Self {
            users: Arc::new(Mutex::new(HashMap::new())),
        }
    }
}

impl UserStore for MemoryStore {
    fn get(&self, id: u64) -> UserResult<User> {
        let users = self.users.lock().unwrap();
        users
            .get(&id)
            .cloned()
            .ok_or(UserError::NotFound(id))
    }

    fn save(&mut self, user: User) -> UserResult<()> {
        let mut users = self.users.lock().unwrap();
        users.insert(user.id, user);
        Ok(())
    }

    fn delete(&mut self, id: u64) -> UserResult<bool> {
        let mut users = self.users.lock().unwrap();
        Ok(users.remove(&id).is_some())
    }
}

/// Async function example
async fn fetch_user(id: u64) -> UserResult<User> {
    // Simulated async operation
    tokio::time::sleep(std::time::Duration::from_millis(100)).await;
    Ok(User::new(id, "Async User", "async@example.com"))
}

/// Generic function with trait bounds
fn process_items<T, I>(items: I) -> Vec<String>
where
    T: Display,
    I: IntoIterator<Item = T>,
{
    items.into_iter().map(|item| item.to_string()).collect()
}

fn main() -> io::Result<()> {
    // Variable bindings
    let mut store = MemoryStore::new();

    // Create users
    let users = vec![
        User::new(1, "Alice", "alice@example.com"),
        User::new(2, "Bob", "bob@example.com"),
        User::new(3, "Charlie", "charlie@example.com"),
    ];

    // Closures
    let get_name = |u: &User| u.name.clone();
    let names: Vec<_> = users.iter().map(get_name).collect();

    // Pattern matching
    for user in &users {
        match user.status {
            Status::Active => println!("{} is active", user.name),
            Status::Inactive => println!("{} is inactive", user.name),
            Status::Pending => println!("{} is pending", user.name),
        }

        // If let
        if let Some(email_domain) = user.email.split('@').nth(1) {
            println!("Domain: {}", email_domain);
        }
    }

    // Iterator chains
    let active_emails: Vec<_> = users
        .iter()
        .filter(|u| u.status == Status::Active)
        .map(|u| &u.email)
        .collect();

    // Macros
    println!("Users: {:?}", names);
    eprintln!("Active emails: {:?}", active_emails);

    // String formatting
    let formatted = format!("Found {} users", users.len());

    // Raw strings
    let raw = r#"This is a "raw" string with \n no escapes"#;
    let raw_multiline = r###"
        Multiple pound signs
        for "nested" quotes
    "###;

    // Byte strings
    let bytes: &[u8] = b"Hello, bytes!";

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_user_creation() {
        let user = User::new(1, "Test", "test@example.com");
        assert_eq!(user.display_name(), "Test");
        assert_eq!(user.status, Status::Active);
    }

    #[test]
    #[should_panic(expected = "not found")]
    fn test_user_not_found() {
        let store = MemoryStore::new();
        store.get(999).expect("not found");
    }
}
