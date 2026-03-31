// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package daemon

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/e2e/client"
)

func TestPropertyAPI(t *testing.T) {
	httpClient := GetHttpclient(t)

	key1, value1 := "key_1", "value_1"
	key2, value2 := "key_2", "value_2"
	updatedValue1 := "value_1_updated"

	t.Run("should start with an empty list", func(t *testing.T) {
		resp, err := httpClient.GetPropertyKeysWithResponse(t.Context())
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.Nil(t, resp.JSON200.Keys, "property list should be empty")
	})
	t.Run("should create two properties", func(t *testing.T) {
		createdProp1 := createProperty(t, httpClient, key1, value1)
		require.Equal(t, value1, string(createdProp1))

		createdProp2 := createProperty(t, httpClient, key2, value2)
		require.Equal(t, value2, string(createdProp2))

		resp, err := httpClient.GetPropertyKeysWithResponse(t.Context())
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.Len(t, *resp.JSON200.Keys, 2)

		keys := []string{key1, key2}
		require.Equal(t, keys, *resp.JSON200.Keys)
	})

	t.Run("should update a property", func(t *testing.T) {
		propertiyBytes := createProperty(t, httpClient, key1, updatedValue1)
		require.Equal(t, updatedValue1, string(propertiyBytes))

		propertiyBytes = getProperty(t, httpClient, key1)
		require.NotNil(t, propertiyBytes, "property should exist")
		require.Equal(t, updatedValue1, string(propertiyBytes))
	})

	t.Run("should delete a property", func(t *testing.T) {

		createdProp1 := createProperty(t, httpClient, key1, value1)
		require.Equal(t, value1, string(createdProp1))

		createdProp2 := createProperty(t, httpClient, key2, value2)
		require.Equal(t, value2, string(createdProp2))

		deleteProperty(t, httpClient, key1)

		prop := getProperty(t, httpClient, key1)
		require.Nil(t, prop, "property should have been deleted")

		resp, err := httpClient.GetPropertyKeysWithResponse(t.Context())
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())

		require.Len(t, *resp.JSON200.Keys, 1)
		keys := []string{key2}
		require.Equal(t, keys, *resp.JSON200.Keys)
	})
}

func createProperty(t *testing.T, httpClient *client.ClientWithResponses, key string, value string) []byte {
	t.Helper()
	reader := io.NopCloser(strings.NewReader(value))

	r, err := httpClient.UpdatePropertyWithBody(t.Context(), key, "application/octet-stream", reader)

	require.NoError(t, err)
	defer r.Body.Close()
	require.Equal(t, http.StatusOK, r.StatusCode)

	actualValue, err := io.ReadAll(r.Body)
	require.NoError(t, err)

	return actualValue
}

func getProperty(t *testing.T, httpClient *client.ClientWithResponses, key string) []byte {
	t.Helper()

	resp, err := httpClient.GetProperty(t.Context(), key)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	require.Equal(t, http.StatusOK, resp.StatusCode)

	actualValue, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return actualValue
}

func deleteProperty(t *testing.T, httpClient *client.ClientWithResponses, key string) {
	t.Helper()

	r, err := httpClient.DeleteProperty(t.Context(), key)
	require.NoError(t, err)
	defer r.Body.Close()
	require.Equal(t, http.StatusNoContent, r.StatusCode)
}
