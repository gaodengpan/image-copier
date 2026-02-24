package output

type FileSystem interface {
	CreateTempFile(pattern string) (string, error)
	RemoveFile(path string) error
}
