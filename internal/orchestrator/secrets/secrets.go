package secrets

import (
	"github.com/arduino/go-paths-helper"
	"github.com/google/renameio/v2"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

func UpdateSecret(cfg config.Configuration, appID app.ID, name string, value []byte) error {
	appPath := cfg.SecretsDir().Join(appID.String())
	_ = appPath.MkdirAll()

	secretPath := appPath.Join(name)
	return renameio.WriteFile(secretPath.String(), value, 0o600)
}

func RemoveSecret(cfg config.Configuration, appID app.ID, name string) error {
	secretPath := cfg.SecretsDir().Join(appID.String(), name)
	if secretPath.NotExist() {
		return nil
	}

	return secretPath.Remove()
}

func ListSecrets(cfg config.Configuration, appID app.ID) ([]string, error) {
	appPath := cfg.SecretsDir().Join(appID.String())
	if appPath.NotExist() {
		return nil, nil
	}

	files, err := appPath.ReadDir(func(p *paths.Path) bool { return !p.IsDir() })
	if err != nil {
		return nil, err
	}

	secrets := make([]string, 0, len(files))
	for _, file := range files {
		secrets = append(secrets, file.Base())
	}

	return secrets, nil
}

type Secret struct {
	Name string
	Path string
}

func GetSecrets(cfg config.Configuration, appID app.ID) ([]Secret, error) {
	appPath := cfg.SecretsDir().Join(appID.String())
	if appPath.NotExist() {
		return nil, nil
	}

	files, err := appPath.ReadDir(func(p *paths.Path) bool { return !p.IsDir() })
	if err != nil {
		return nil, err
	}

	secrets := make([]Secret, 0, len(files))
	for _, file := range files {
		secrets = append(secrets, Secret{
			Name: file.Base(),
			Path: file.String(),
		})
	}

	return secrets, nil
}
