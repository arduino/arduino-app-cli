// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package secrets manages the App secrets outside the app folder.
package secrets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/config"
)

// Store is a secret storage for an app, managing its secrets outside the app folder.
type Store struct {
	file *paths.Path
}

// NewStore creates a new secret store manager based on the given configuration and app.
func NewStore(cfg config.Configuration, appID appid.ID) *Store {
	// secretsDir is the directory of app secrets.
	secretsDir := cfg.DataDir().Join("secrets")

	// The app ID is hashed to avoid collisions between apps.
	sum := sha256.Sum256([]byte(appID.String()))
	return &Store{file: secretsDir.Join(hex.EncodeToString(sum[:]) + ".yaml")}
}

// Get return the secrects of an app. An app without secrets will return an empty map.
func (s *Store) Get() (map[string]string, error) {
	values := map[string]string{}

	// Read secrets from disk
	data, err := s.file.ReadFile()
	if err != nil {
		if os.IsNotExist(err) {
			// If the secrets file does not exist, return an empty map.
			return values, nil
		}
		return nil, err
	}

	// Unmarshal the secrets from the YAML file.
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, err
	}

	// Check if secrets names are valid
	for name := range values {
		if !secretNameIsValid(name) {
			return nil, fmt.Errorf("invalid secret name: %s", name)
		}
	}

	return values, nil
}

// Set writes the secrets of an app. Only the secrets in the map will be saved to disk.
// Existing secrets not included in the map will be removed from the store.
func (s *Store) Set(updates map[string]string) error {
	if len(updates) == 0 {
		return s.file.RemoveAll()
	}

	// Check if secrets names are valid
	for name := range updates {
		if !secretNameIsValid(name) {
			return fmt.Errorf("invalid secret name: %s", name)
		}
	}

	// Marshal the secrets to YAML format.
	data, err := yaml.Marshal(updates)
	if err != nil {
		return err
	}

	// Ensure the secrets directory exists.
	if err := s.file.Parent().MkdirAll(); err != nil {
		return err
	}

	// Write the secrets to disk.
	// Only the user running the App CLI can read the secrets.
	const secretsFileMode os.FileMode = 0600
	s.file.Create()
	f, err := os.OpenFile(s.file.String(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, secretsFileMode)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

// Delete removes all secrets of an app.
func (s *Store) Delete() error {
	return s.Set(nil)
}

var secretNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// secretNameIsValid checks if the secret name is allowed.
func secretNameIsValid(name string) bool {
	return secretNameRegex.MatchString(name)
}
