package output

// ImageIDService provides image ID manipulation methods
type ImageIDService interface {
	// BuildDestImageID builds the destination image ID for the staging registry
	BuildDestImageID(sourceID, registryHost, registryNamespace string) string

	// NormalizeSourceID normalizes a source image ID
	NormalizeSourceID(imageID string) string
}