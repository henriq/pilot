package ports

type AccessMode int

const (
	ReadWrite = iota
	ReadWriteExecute
	ReadAllWriteOwner
)

type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte, accessMode AccessMode) error
	EnsureDirExists(path string) error
	FileExists(path string) (bool, error)
	MkdirAll(path string, accessMode AccessMode) error
	RemoveAll(path string) error
	ReadSubdirectories(path string) ([]string, error)
	DirSize(path string) (int64, error)
	// HomeDir returns the user's home directory path.
	// Used when paths need to be expanded for external tools like Helm.
	HomeDir() (string, error)
}
