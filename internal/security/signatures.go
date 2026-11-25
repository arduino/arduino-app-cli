// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// Package security verifies the detached GPG signatures of the files downloaded from Arduino.
// It is ported from the security package of arduino-cli.
package security

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/arduino/go-paths-helper"
)

//go:embed keys/arduino_public.gpg.key
var arduinoPublicKey []byte

// VerifyArduinoDetachedSignature checks that the detached GPG signature (in the
// signaturePath file) matches the given targetPath file and is an authentic
// signature from the bundled Arduino public key. If any of the above conditions
// fails this function returns false. The PGP entity in the trusted keychain that
// produced the signature is returned too.
func VerifyArduinoDetachedSignature(targetPath *paths.Path, signaturePath *paths.Path) (bool, *openpgp.Entity, error) {
	return VerifySignature(targetPath, signaturePath, bytes.NewReader(arduinoPublicKey))
}

// VerifyDetachedSignature checks that the detached GPG signature (in the
// signaturePath file) matches the given targetPath file and is an authentic
// signature from the public key in keyPath.
// If any of the above conditions fails this function returns false.
// The PGP entity in the trusted keychain that produced the signature is returned too.
func VerifyDetachedSignature(targetPath *paths.Path, signaturePath *paths.Path, keyPath *paths.Path) (bool, *openpgp.Entity, error) {
	keyring, err := keyPath.Open()
	if err != nil {
		return false, nil, fmt.Errorf("opening signature keys: %w", err)
	}
	defer keyring.Close()
	return VerifySignature(targetPath, signaturePath, keyring)
}

// VerifySignature checks that the detached GPG signature (in the
// signaturePath file) matches the given targetPath file and is an authentic
// signature from one of the keys read from keyring.
//
// If any of the above conditions fails this function returns false.
//
// The PGP entity in the trusted keychain that produced the signature is returned too.
func VerifySignature(targetPath *paths.Path, signaturePath *paths.Path, keyring io.Reader) (bool, *openpgp.Entity, error) {
	keyRing, err := openpgp.ReadKeyRing(keyring)
	if err != nil {
		return false, nil, fmt.Errorf("retrieving the public keys: %w", err)
	}
	target, err := targetPath.ReadFile()
	if err != nil {
		return false, nil, fmt.Errorf("opening target file: %w", err)
	}
	signature, err := signaturePath.ReadFile()
	if err != nil {
		return false, nil, fmt.Errorf("opening signature file: %w", err)
	}
	signer, err := openpgp.CheckDetachedSignature(keyRing, bytes.NewReader(target), bytes.NewReader(signature), nil)

	if errors.Is(err, pgperrors.ErrSignatureExpired) {
		err = errors.New("signature expired: is your system clock set correctly?")
	}

	return (signer != nil && err == nil), signer, err
}
