package ports

// SecretStore abstracts Kubernetes secret read operations for certificate status checks.
type SecretStore interface {
	// GetSecretData returns the data from a Kubernetes secret by name.
	// Returns (nil, nil) if the secret does not exist.
	GetSecretData(name string) (map[string][]byte, error)
}
