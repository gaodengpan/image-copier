package ports

type FileSystem interface {
	CreateTempFile(pattern string) (string, error)
	RemoveFile(path string) error
}
