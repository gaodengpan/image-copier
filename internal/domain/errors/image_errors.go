package errors

import "fmt"

type ImageErrorCode int

const (
	ErrInvalidImageName ImageErrorCode = iota
	ErrImageNotFound
	ErrWorkflowTriggerFailed
	ErrWorkflowTimeout
	ErrImageDownloadFailed
	ErrImageLoadFailed
	ErrValidationFailed
)

type ImageError struct {
	code    ImageErrorCode
	imageID string
	message string
	cause   error
}

func NewImageError(code ImageErrorCode, imageID, message string) *ImageError {
	return &ImageError{
		code:    code,
		imageID: imageID,
		message: message,
	}
}

func NewImageErrorWithCause(code ImageErrorCode, imageID, message string, cause error) *ImageError {
	return &ImageError{
		code:    code,
		imageID: imageID,
		message: message,
		cause:   cause,
	}
}

func (e *ImageError) Code() ImageErrorCode {
	return e.code
}

func (e *ImageError) ImageID() string {
	return e.imageID
}

func (e *ImageError) Message() string {
	return e.message
}

func (e *ImageError) Cause() error {
	return e.cause
}

func (e *ImageError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("image error [%d] %s (image: %s): %v", e.code, e.message, e.imageID, e.cause)
	}
	return fmt.Sprintf("image error [%d] %s (image: %s)", e.code, e.message, e.imageID)
}

func (e *ImageError) Unwrap() error {
	return e.cause
}
