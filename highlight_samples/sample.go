// Sample Go code for syntax highlighting
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Constants
const (
	DefaultTimeout = 30 * time.Second
	MaxRetries     = 3
)

// User represents a user in the system.
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// UserService handles user operations.
type UserService interface {
	GetUser(ctx context.Context, id int) (*User, error)
	CreateUser(ctx context.Context, user *User) error
}

// userServiceImpl implements UserService.
type userServiceImpl struct {
	mu    sync.RWMutex
	users map[int]*User
}

// NewUserService creates a new UserService.
func NewUserService() UserService {
	return &userServiceImpl{
		users: make(map[int]*User),
	}
}

// GetUser retrieves a user by ID.
func (s *userServiceImpl) GetUser(ctx context.Context, id int) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return nil, fmt.Errorf("user %d not found", id)
	}
	return user, nil
}

// CreateUser adds a new user.
func (s *userServiceImpl) CreateUser(ctx context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[user.ID]; exists {
		return fmt.Errorf("user %d already exists", user.ID)
	}

	user.CreatedAt = time.Now()
	s.users[user.ID] = user
	return nil
}

func main() {
	// Initialize service
	svc := NewUserService()

	// Create sample users
	users := []*User{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
	}

	ctx := context.Background()
	for _, u := range users {
		if err := svc.CreateUser(ctx, u); err != nil {
			log.Printf("Failed to create user: %v", err)
			continue
		}
		fmt.Printf("Created user: %s\n", u.Name)
	}

	// Goroutines and channels
	ch := make(chan *User, len(users))
	var wg sync.WaitGroup

	for _, u := range users {
		wg.Add(1)
		go func(user *User) {
			defer wg.Done()
			ch <- user
		}(u)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for user := range ch {
		data, _ := json.Marshal(user)
		fmt.Println(string(data))
	}

	// HTTP handler example
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Println("Starting server on :8080")
}
