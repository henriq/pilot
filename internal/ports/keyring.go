package ports

type Keyring interface {
	GetKey(keyName string) (string, error)
	SetKey(keyName string, keyValue string) error
	HasKey(keyName string) (bool, error)
	DeleteKey(keyName string) error
}
