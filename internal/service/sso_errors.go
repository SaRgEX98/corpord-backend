package service

import "errors"

var ErrSSOAccountExists = errors.New("account already exists; use its existing login method")
