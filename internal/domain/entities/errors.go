package entities

import "errors"

var (
	ErrTaskAlreadyStarted = errors.New("task already started")
	ErrTaskNotSyncing     = errors.New("task not in syncing status")
)
