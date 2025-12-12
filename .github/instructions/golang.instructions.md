---
applyTo: "**/*.go"
---

## Go Coding Conventions

### Return Early (Circuit Breaker Pattern)

Early returns help reduce nesting and improve code readability by handling edge cases first.

**Do:**
```go
func ProcessUserData(ctx context.Context, user *User) (*ProcessedData, error) {
	if user == nil {
		return nil, errors.New("user is nil")
	}
	if !user.IsActive {
		return nil, errors.New("user is inactive")
	}
	if user.Role != "admin" && user.AccessLevel < 3 {
		return nil, errors.New("insufficient permissions")
	}

	// Main logic here after all validations pass
	return performProcessing(ctx, user)
}
```

**Don't:**
```go
func ProcessUserData(ctx context.Context, user *User) (*ProcessedData, error) {
	if user != nil {
		if user.IsActive {
			if user.Role == "admin" || user.AccessLevel >= 3 {
				// Main logic deeply nested
				return performProcessing(ctx, user)
			} else {
				return nil, errors.New("insufficient permissions")
			}
		} else {
			return nil, errors.New("user is inactive")
		}
	} else {
		return nil, errors.New("user is nil")
	}
}
```

### Avoid Using Else & Use Default Values

Minimizing `else` statements improves readability and reduces complexity. Use default values and early returns to handle common cases.

**Do:**
```go
func GetUserStatus(ctx context.Context, user *User) UserStatus {
	// Default value
	status := UserStatus{
		State:    "offline",
		LastSeen: nil,
	}

	if user == nil {
		return status
	}

	if user.Online {
		return UserStatus{
			State:    "online",
			LastSeen: time.Now(),
		}
	}

	if user.LastActivity != nil {
		return UserStatus{
			State:    "away",
			LastSeen: user.LastActivity,
		}
	}

	return status
}
```

**Don't:**
```go
func GetUserStatus(ctx context.Context, user *User) UserStatus {
	if user != nil {
		if user.Online {
			return UserStatus{
				State:    "online",
				LastSeen: time.Now(),
			}
		} else {
			if user.LastActivity != nil {
				return UserStatus{
					State:    "away",
					LastSeen: user.LastActivity,
				}
			} else {
				return UserStatus{
					State:    "offline",
					LastSeen: nil,
				}
			}
		}
	} else {
		return UserStatus{
			State:    "offline",
			LastSeen: nil,
		}
	}
}
```

### Avoid Using Named Return Values

Named return values can make code harder to read and understand, especially in longer functions. Use explicit returns for clarity.

**Do:**
```go
func ProcessData(ctx context.Context, data []byte) (*Result, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	result, err := parseData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data: %w", err)
	}

	return result, nil
}
```

**Don't:**
```go
func ProcessData(ctx context.Context, data []byte) (result *Result, err error) {
	if len(data) == 0 {
		err = errors.New("empty data")
		return
	}

	result, err = parseData(data)
	if err != nil {
		err = fmt.Errorf("failed to parse data: %w", err)
		return
	}

	return
}
```

### Separate Logic and Data

Keeping logic and data separate makes code more maintainable and testable. In Go, this often means separating into different packages.

**Do:**
```go
// models/roles.go
package models

// RolePermissions defines what actions a role can perform
type RolePermissions struct {
	CanEdit   bool
	CanDelete bool
	CanInvite bool
}

// RoleMap defines available roles and their permissions
var RoleMap = map[string]RolePermissions{
	"admin":  {CanEdit: true, CanDelete: true, CanInvite: true},
	"editor": {CanEdit: true, CanDelete: false, CanInvite: false},
	"viewer": {CanEdit: false, CanDelete: false, CanInvite: false},
}

// services/permissions.go
package services

import (
	"context"
	"myapp/models"
)

// PermissionChecker handles permission verification
type PermissionChecker struct {
	// Dependencies injected via constructor
}

// CheckPermission verifies if a user can perform an action
func (p *PermissionChecker) CheckPermission(ctx context.Context, user *models.User, action string) bool {
	if user == nil || user.Role == "" {
		return false
	}

	permissions, exists := models.RoleMap[user.Role]
	if !exists {
		// Default to viewer permissions if role not found
		permissions = models.RoleMap["viewer"]
	}

	switch action {
	case "edit":
		return permissions.CanEdit
	case "delete":
		return permissions.CanDelete
	case "invite":
		return permissions.CanInvite
	default:
		return false
	}
}
```

### Interface Methods Should Include Context as First Argument

All interface methods should include a context parameter as the first argument to support timeout, cancellation, and request-scoped values.

**Do:**
```go
// UserRepository defines methods for user data access
type UserRepository interface {
	GetByID(ctx context.Context, id string) (*User, error)
	Save(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
}

// Implementation
type SQLUserRepository struct {
	db *sql.DB
}

func (r *SQLUserRepository) GetByID(ctx context.Context, id string) (*User, error) {
	// Use ctx for query timeouts, cancellation
	row := r.db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = ?", id)
	// ... rest of implementation
}
```

### Avoid Technology Names in Main Business Flow

If you spot technology names or specific component names in your main business flow, it's not the way to go. Business logic should be abstracted and not coupled to specific technologies.

**Do:**
```go
// Main business flow - no technology names visible
func ProcessUserRegistration(ctx context.Context, userData UserRegistrationData) error {
	// Validate user data
	if err := validateRegistrationData(userData); err != nil {
		return err
	}

	// Create user account
	user, err := createUserAccount(ctx, userData)
	if err != nil {
		return err
	}

	// Send welcome notification
	if err := sendWelcomeMessage(ctx, user); err != nil {
		// Log error but don't fail the registration
		log.Error("failed to send welcome message", "user_id", user.ID, "error", err)
	}

	// Provision access
	return provisionUserAccess(ctx, user)
}

// Technology-specific implementations are abstracted away
type NotificationService interface {
	SendWelcome(ctx context.Context, user *User) error
}

type AccessProvisionService interface {
	ProvisionAccess(ctx context.Context, user *User) error
}
```

**Don't:**
```go
// Bad - technology names leak into business logic
func ProcessUserRegistration(ctx context.Context, userData UserRegistrationData) error {
	// Validate user data
	if err := validateRegistrationData(userData); err != nil {
		return err
	}

	// Create user account
	user, err := createUserAccount(ctx, userData)
	if err != nil {
		return err
	}

	// Send email via SendGrid - technology name in business flow!
	if err := sendGridClient.SendWelcomeEmail(ctx, user.Email); err != nil {
		log.Error("SendGrid failed", "error", err)
	}

	// Provision in Active Directory - technology name in business flow!
	if err := activeDirectoryClient.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("AD provisioning failed: %w", err)
	}

	// Update Salesforce - technology name in business flow!
	return salesforceClient.CreateContact(ctx, user)
}
```

### Don't Mix Standard Logic and Technical Integrations

Separate core business logic from technical integrations like SailPoint. Use the observer pattern to keep these concerns decoupled.

**Do:**
```go
// domain/user_service.go
package domain

import "context"

// UserService handles core user business logic
type UserService struct {
	repo           UserRepository
	eventPublisher EventPublisher
}

func NewUserService(repo UserRepository, publisher EventPublisher) *UserService {
	return &UserService{
		repo:           repo,
		eventPublisher: publisher,
	}
}

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
	// Core business logic
	if err := validateUser(user); err != nil {
		return err
	}

	// Save to database
	if err := s.repo.Save(ctx, user); err != nil {
		return err
	}

	// Publish event for observers (like SailPoint) to react to
	s.eventPublisher.Publish(ctx, "user.created", user)

	return nil
}

// integrations/sailpoint/observer.go
package sailpoint

import "context"

// SailPointObserver implements the Observer interface
type SailPointObserver struct {
	client SailPointClient
}

func (o *SailPointObserver) OnEvent(ctx context.Context, eventType string, data interface{}) {
	switch eventType {
	case "user.created":
		user, ok := data.(*User)
		if ok {
			o.client.SyncUser(ctx, user)
		}
	// Handle other events
	}
}
```

### Everything Goes Through IoC (Dependency Injection) Using Dig

We use [go-dig](https://github.com/uber-go/dig) for dependency injection in Go projects.

**Do:**
```go
import (
	"go.uber.org/dig"
)

// Services with constructor-based dependency injection
type UserController struct {
	userService UserService
	authService AuthService
}

func NewUserController(userService UserService, authService AuthService) *UserController {
	return &UserController{
		userService: userService,
		authService: authService,
	}
}

// In main.go or setup.go
func BuildContainer() *dig.Container {
	container := dig.New()

	// Register dependencies
	container.Provide(initDatabase)
	container.Provide(NewEventBus)
	container.Provide(NewSQLUserRepository)
	container.Provide(NewAuthService)
	container.Provide(NewUserService)
	container.Provide(NewUserController)

	return container
}

func StartApp() {
	container := BuildContainer()

	// Invoke the application entry point
	err := container.Invoke(func(controller *UserController) {
		// Start using the controller
		server := NewServer(controller)
		server.Start()
	})

	if err != nil {
		panic(err)
	}
}
```

### Testing: Using Standard Lib, Use Require (Not Assert)

Use the standard library for testing when possible, and prefer `require` over `assert` to fail fast.

**Do:**
```go
func TestUserService(t *testing.T) {
	// Setup
	mockRepo := &MockUserRepository{}
	mockEventBus := &MockEventPublisher{}
	service := NewUserService(mockRepo, mockEventBus)

	// Define expectations
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*User")).Return(nil)
	mockEventBus.On("Publish", mock.Anything, "user.created", mock.AnythingOfType("*User")).Return()

	// Execute test
	ctx := context.Background()
	user := &User{Name: "Test User"}
	err := service.CreateUser(ctx, user)

	// Assertions
	require.NoError(t, err) // Fails immediately if error exists
	mockRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}
```

### Configuration with godotenv and Dependency Injection

Use `godotenv` to load environment files and `caarlos0/env` to parse configuration. Load once at startup and distribute via DI.

**Do:**
```go
// config/config.go
type Config struct {
	Server ServerConfig
	Auth   AuthConfig
}

type ServerConfig struct {
	Port int `env:"SERVER_PORT" envDefault:"8080"`
}

type AuthConfig struct {
	ClientID string `env:"AUTH_CLIENT_ID"`
	Secret   string `env:"AUTH_CLIENT_SECRET"`
}

// DepConfig for dependency injection
type DepConfig struct {
	dig.Out
	*ServerConfig
	*AuthConfig
}

func Load() *Config {
	for _, file := range []string{".env", ".env.defaults", ".env.secrets"} {
		_ = godotenv.Load(file)
	}
	var cfg Config
	env.Parse(&cfg)
	return &cfg
}

func ParseDependenciesConfig(cfg *Config) DepConfig {
	return DepConfig{dig.Out{}, &cfg.Server, &cfg.Auth}
}

// main.go
container.Provide(config.Load)
container.Provide(config.ParseDependenciesConfig)
container.Provide(func(cfg *config.AuthConfig) *auth.Service {
	return auth.NewService(cfg)
})
```

**Don't:**
```go
// Bad - reading env vars directly in business logic
host := os.Getenv("HOST")

// Bad - global config accessed everywhere
var GlobalConfig *Config
```

### Use Mockery for Test Mocks

Always use mockery-generated mocks for testing. Never create manual mocks.

**Do:**
```go
func TestUserService(t *testing.T) {
	// Use generated mock
	mockRepo := mocks.NewMockUserRepository(t)
	mockEventBus := mocks.NewMockEventPublisher(t)

	// Setup expectations with EXPECT()
	mockRepo.EXPECT().Save(mock.Anything, mock.AnythingOfType("*User")).Return(nil)
	mockEventBus.EXPECT().Publish(mock.Anything, "user.created", mock.AnythingOfType("*User")).Return()

	service := NewUserService(mockRepo, mockEventBus)

	// Execute test
	ctx := context.Background()
	user := &User{Name: "Test User"}
	err := service.CreateUser(ctx, user)

	// Verify
	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}
```

**Don't:**
```go
// Bad - manual mock implementation
type mockUserRepository struct {
	saveFunc func(ctx context.Context, user *User) error
}

func (m *mockUserRepository) Save(ctx context.Context, user *User) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, user)
	}
	return nil
}
```

**Regenerate mocks after interface changes:**
```bash
make mocks
```
