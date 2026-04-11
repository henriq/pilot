package handler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pilot/internal/testutil"
)

const sampleConfig = `contexts:
  - name: my-context
    services:
      - name: my-service
  - name: other-context
    services:
      - name: other-service
`

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
}

func newTestHandler(t *testing.T, home string, answer string, oldKeyring *testutil.MockKeyring, newKeyring *testutil.MockKeyring) *MigrateCommandHandler {
	t.Helper()
	terminal := &testutil.MockTerminalInput{}
	terminal.On("IsTerminal").Return(true)
	terminal.On("ReadLine", mock.Anything).Return(answer, nil)
	fileSystem := &testutil.MockFileSystem{}
	fileSystem.On("HomeDir").Return(home, nil)
	handler := NewMigrateCommandHandler(oldKeyring, newKeyring, terminal, fileSystem)
	return &handler
}

func TestMigrateCommandHandler_Handle_NoOldConfig(t *testing.T) {
	home := t.TempDir()
	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}
	sut := newTestHandler(t, home, "y", oldKr, newKr)

	migrated, err := sut.Handle()

	require.NoError(t, err)
	assert.False(t, migrated)
}

func TestMigrateCommandHandler_Handle_NewConfigAlreadyExists(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)
	writeTestFile(t, filepath.Join(home, newConfigFileName), "existing")
	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}
	sut := newTestHandler(t, home, "y", oldKr, newKr)

	migrated, err := sut.Handle()

	require.NoError(t, err)
	assert.False(t, migrated)
}

func TestMigrateCommandHandler_Handle_SkipsInNonInteractiveMode(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)
	terminal := &testutil.MockTerminalInput{}
	terminal.On("IsTerminal").Return(false)
	fileSystem := &testutil.MockFileSystem{}
	fileSystem.On("HomeDir").Return(home, nil)
	sut := &MigrateCommandHandler{
		oldKeyring:    &testutil.MockKeyring{},
		newKeyring:    &testutil.MockKeyring{},
		terminalInput: terminal,
		fileSystem:    fileSystem,
	}

	migrated, err := sut.Handle()

	require.NoError(t, err)
	assert.False(t, migrated)
	terminal.AssertNotCalled(t, "ReadLine", mock.Anything)
}

func TestMigrateCommandHandler_Handle_UserDeclines(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)
	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}
	sut := newTestHandler(t, home, "n", oldKr, newKr)

	migrated, err := sut.Handle()

	require.NoError(t, err)
	assert.False(t, migrated)
	assert.FileExists(t, filepath.Join(home, oldConfigFileName))
	assert.NoFileExists(t, filepath.Join(home, newConfigFileName))
}

func TestMigrateCommandHandler_Handle_MigratesConfigFile(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)
	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}
	oldKr.On("GetKey", mock.Anything).Return("", assert.AnError)
	sut := newTestHandler(t, home, "y", oldKr, newKr)

	migrated, err := sut.Handle()

	require.NoError(t, err)
	assert.True(t, migrated)

	data, err := os.ReadFile(filepath.Join(home, newConfigFileName)) //nolint:gosec // test file
	require.NoError(t, err)
	assert.Equal(t, sampleConfig, string(data))

	assert.NoFileExists(t, filepath.Join(home, oldConfigFileName))
	oldKr.AssertExpectations(t)
}

func TestMigrateCommandHandler_Handle_MigratesDataDirectory(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)

	oldDataDir := filepath.Join(home, oldDataDirName)
	writeTestFile(t, filepath.Join(oldDataDir, "my-context", "secrets"), "encrypted-data")
	writeTestFile(t, filepath.Join(oldDataDir, "current-context"), "my-context")

	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}
	oldKr.On("GetKey", mock.Anything).Return("", assert.AnError)
	sut := newTestHandler(t, home, "y", oldKr, newKr)

	migrated, err := sut.Handle()

	require.NoError(t, err)
	assert.True(t, migrated)

	assert.NoDirExists(t, oldDataDir)

	newDataDir := filepath.Join(home, newDataDirName)
	data, err := os.ReadFile(filepath.Join(newDataDir, "my-context", "secrets")) //nolint:gosec // test file
	require.NoError(t, err)
	assert.Equal(t, "encrypted-data", string(data))

	data, err = os.ReadFile(filepath.Join(newDataDir, "current-context")) //nolint:gosec // test file
	require.NoError(t, err)
	assert.Equal(t, "my-context", string(data))
	oldKr.AssertExpectations(t)
}

func TestMigrateCommandHandler_Handle_MigratesKeyringSecrets(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)

	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}

	oldKr.On("GetKey", "my-context-encryption-key").Return("enc-key-1", nil)
	oldKr.On("GetKey", "my-context-ca-key").Return("ca-key-1", nil)
	oldKr.On("GetKey", "other-context-encryption-key").Return("enc-key-2", nil)
	oldKr.On("GetKey", "other-context-ca-key").Return("", assert.AnError)
	oldKr.On("DeleteKey", "my-context-encryption-key").Return(nil)
	oldKr.On("DeleteKey", "my-context-ca-key").Return(nil)
	oldKr.On("DeleteKey", "other-context-encryption-key").Return(nil)

	newKr.On("SetKey", "my-context-encryption-key", "enc-key-1").Return(nil)
	newKr.On("SetKey", "my-context-ca-key", "ca-key-1").Return(nil)
	newKr.On("SetKey", "other-context-encryption-key", "enc-key-2").Return(nil)

	sut := newTestHandler(t, home, "y", oldKr, newKr)

	migrated, err := sut.Handle()

	require.NoError(t, err)
	assert.True(t, migrated)
	oldKr.AssertExpectations(t)
	newKr.AssertExpectations(t)
}

func TestMigrateCommandHandler_Handle_NoDataDirToMigrate(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)

	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}
	oldKr.On("GetKey", mock.Anything).Return("", assert.AnError)
	sut := newTestHandler(t, home, "y", oldKr, newKr)

	migrated, err := sut.Handle()

	require.NoError(t, err)
	assert.True(t, migrated)
	assert.FileExists(t, filepath.Join(home, newConfigFileName))
}

func TestMigrateCommandHandler_Handle_FailsIfNewDataDirExists(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)
	writeTestFile(t, filepath.Join(home, oldDataDirName, "file"), "old")
	writeTestFile(t, filepath.Join(home, newDataDirName, "file"), "new")

	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}
	sut := newTestHandler(t, home, "y", oldKr, newKr)

	_, err := sut.Handle()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.FileExists(t, filepath.Join(home, oldConfigFileName))
	assert.NoFileExists(t, filepath.Join(home, newConfigFileName))
}

func TestMigrateCommandHandler_Handle_NoKeyringKeysToMigrate(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)

	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}
	oldKr.On("GetKey", mock.Anything).Return("", assert.AnError)
	sut := newTestHandler(t, home, "y", oldKr, newKr)

	migrated, err := sut.Handle()

	require.NoError(t, err)
	assert.True(t, migrated)
}

func TestMigrateCommandHandler_Handle_ErrorReadingUserInput(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), sampleConfig)

	terminal := &testutil.MockTerminalInput{}
	terminal.On("IsTerminal").Return(true)
	terminal.On("ReadLine", mock.Anything).Return("", assert.AnError)
	fileSystem := &testutil.MockFileSystem{}
	fileSystem.On("HomeDir").Return(home, nil)
	sut := &MigrateCommandHandler{
		oldKeyring:    &testutil.MockKeyring{},
		newKeyring:    &testutil.MockKeyring{},
		terminalInput: terminal,
		fileSystem:    fileSystem,
	}

	migrated, err := sut.Handle()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read user input")
	assert.False(t, migrated)
}

func TestMigrateCommandHandler_Handle_ErrorParsingMalformedConfig(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, oldConfigFileName), "not: [valid: yaml")

	oldKr := &testutil.MockKeyring{}
	newKr := &testutil.MockKeyring{}
	sut := newTestHandler(t, home, "y", oldKr, newKr)

	migrated, err := sut.Handle()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse old config file")
	assert.False(t, migrated)
}
