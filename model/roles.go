package model

// User roles
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// ValidRoles is a map of all valid roles for validation
var ValidRoles = map[string]bool{
	RoleAdmin: true,
	RoleUser:  true,
}
