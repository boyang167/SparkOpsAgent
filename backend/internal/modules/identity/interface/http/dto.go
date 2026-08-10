package http

type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthTokenResponse struct {
	Token string `json:"token"`
}

type RegisterResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type LoginResponse struct {
	Token string `json:"token"`
	ID    string `json:"id"`
	Email string `json:"email"`
}

type UpdateProfileRequest struct {
	DisplayName string `json:"display_name" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}
