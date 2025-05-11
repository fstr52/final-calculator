package db

import "errors"

var ErrUserAlreadyExists = errors.New("user with this username already exists")
var ErrSQL = errors.New("SQL error")
