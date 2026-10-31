package model

// ValidationError describes invalid client input, without including database details.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalidInput(message string) error { return &ValidationError{Message: message} }
