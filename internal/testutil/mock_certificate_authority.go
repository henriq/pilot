package testutil

import (
	"time"

	"dx/internal/core/domain"
	"dx/internal/ports"

	"github.com/stretchr/testify/mock"
)

var _ ports.CertificateAuthority = (*MockCertificateAuthority)(nil)

type MockCertificateAuthority struct {
	mock.Mock
}

func (m *MockCertificateAuthority) IssueCertificate(contextName string, passphrase string, request domain.CertificateRequest) (*domain.IssuedCertificate, error) {
	args := m.Called(contextName, passphrase, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IssuedCertificate), args.Error(1)
}

func (m *MockCertificateAuthority) GetCACertificatePEM(contextName string) ([]byte, error) {
	args := m.Called(contextName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCertificateAuthority) GetCACertificateExpiry(contextName string) (*time.Time, error) {
	args := m.Called(contextName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockCertificateAuthority) DeleteCA(contextName string) error {
	args := m.Called(contextName)
	return args.Error(0)
}
