package user

// CreateUserRequest represents the data needed to create a new user
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=8"`
}

// UpdateUserRequest represents the data that can be updated for a user
type UpdateUserRequest struct {
	Email    *string `json:"email" validate:"omitempty,email"`
	Username *string `json:"username" validate:"omitempty,min=3,max=50"`
	IsActive *bool   `json:"is_active"`
}
