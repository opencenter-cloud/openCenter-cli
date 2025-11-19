/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crypto

import (
	"context"
	"time"
)

// KeyGenerator interface for generating cryptographic keys
type KeyGenerator interface {
	GenerateAgeKey() (*AgeKeyPair, error)
	GenerateRandomPassword(length int) (string, error)
	GenerateFallbackKey() (*AgeKeyPair, error)
}

// KeyValidator interface for validating cryptographic keys
type KeyValidator interface {
	ValidateAgeKey(key string) error
	ValidatePGPKey(key string) error
	ValidateKeyAccess(keyName string) error
	ValidateKeyForProduction(key string) error
}

// KeyManager interface for managing cryptographic keys
type KeyManager interface {
	KeyGenerator
	KeyValidator
	LoadAgeKey(keyName string) (*AgeKeyPair, error)
	SaveAgeKey(keyPair *AgeKeyPair, keyName string) error
	ListAgeKeys() ([]string, error)
	DeleteAgeKey(keyName string) error
	ImportAgeKey(keyName, privateKey string) (*AgeKeyPair, error)
	ExportAgeKey(keyName string) (*AgeKeyPair, error)
	BackupKeys(backupPath string) error
	RestoreKeys(backupPath string) error
	GetKeyInfo(keyName string) (*KeyInfo, error)
	SetupAgeEnvironment(keyName string) error
	GenerateKeyForCluster(clusterName string) (*AgeKeyPair, error)
	CheckAgeInstallation(ctx context.Context) error
}

// AgeKeyPair represents an age key pair
type AgeKeyPair struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	Recipient  string `json:"recipient"`
}

// KeyInfo represents information about a key
type KeyInfo struct {
	Name      string    `json:"name"`
	PublicKey string    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
	KeyType   string    `json:"key_type"`
	FilePath  string    `json:"file_path"`
}
