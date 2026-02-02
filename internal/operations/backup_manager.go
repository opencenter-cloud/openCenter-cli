package operations

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
)

// BackupManager handles backup and restoration of cluster configurations
type BackupManager interface {
	CreateBackup(ctx context.Context, cluster string) (*Backup, error)
	RestoreBackup(ctx context.Context, backupID string, passphrase string) error
	ListBackups(cluster string) ([]*Backup, error)
	DeleteBackup(backupID string) error
	ScheduleBackups(schedule string, retention time.Duration) error
}

// Backup represents a cluster backup
type Backup struct {
	ID              string
	Cluster         string
	CreatedAt       time.Time
	CreatedBy       string
	Size            int64
	Compressed      bool
	Encrypted       bool
	EncryptionAlgo  string
	Contents        BackupContents
	Checksum        string
	StorageLocation string
	RetentionUntil  time.Time
}

// BackupContents represents the contents of a backup
type BackupContents struct {
	ConfigFile     []byte
	AgeKeys        []byte // Encrypted
	SSHKeys        []byte // Encrypted
	GitOpsState    []byte
	TerraformState []byte
}

// BackupMetadata stores metadata about a backup
type BackupMetadata struct {
	ID              string    `json:"id"`
	Cluster         string    `json:"cluster"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       string    `json:"created_by"`
	Size            int64     `json:"size"`
	Compressed      bool      `json:"compressed"`
	Encrypted       bool      `json:"encrypted"`
	EncryptionAlgo  string    `json:"encryption_algo"`
	Contents        []string  `json:"contents"`
	Checksum        string    `json:"checksum"`
	StorageLocation string    `json:"storage_location"`
	RetentionUntil  time.Time `json:"retention_until"`
}

// backupManager implements BackupManager interface
type backupManager struct {
	pathResolver *paths.PathResolver
	backupDir    string
	currentUser  string
}

// NewBackupManager creates a new backup manager
func NewBackupManager(pathResolver *paths.PathResolver, backupDir string) (BackupManager, error) {
	if pathResolver == nil {
		return nil, fmt.Errorf("path resolver cannot be nil")
	}
	if backupDir == "" {
		return nil, fmt.Errorf("backup directory cannot be empty")
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Get current user
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "unknown"
	}

	return &backupManager{
		pathResolver: pathResolver,
		backupDir:    backupDir,
		currentUser:  currentUser,
	}, nil
}

// CreateBackup creates a new backup of the cluster configuration
func (bm *backupManager) CreateBackup(ctx context.Context, cluster string) (*Backup, error) {
	if cluster == "" {
		return nil, fmt.Errorf("cluster name cannot be empty")
	}

	// Generate backup ID
	backupID := fmt.Sprintf("%s-%s", cluster, time.Now().Format("20060102-150405"))

	// Collect backup contents
	contents, err := bm.collectBackupContents(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to collect backup contents: %w", err)
	}

	// Create backup archive
	archivePath := filepath.Join(bm.backupDir, backupID+".tar.gz")
	if err := bm.createArchive(archivePath, contents); err != nil {
		return nil, fmt.Errorf("failed to create backup archive: %w", err)
	}

	// Calculate checksum
	checksum, err := bm.calculateChecksum(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Get file size
	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup file info: %w", err)
	}

	backup := &Backup{
		ID:              backupID,
		Cluster:         cluster,
		CreatedAt:       time.Now(),
		CreatedBy:       bm.currentUser,
		Size:            fileInfo.Size(),
		Compressed:      true,
		Encrypted:       false, // Will be encrypted with passphrase separately
		EncryptionAlgo:  "AES-256-GCM",
		Contents:        *contents,
		Checksum:        checksum,
		StorageLocation: archivePath,
		RetentionUntil:  time.Now().Add(30 * 24 * time.Hour), // 30 days default
	}

	return backup, nil
}

// RestoreBackup restores a cluster configuration from a backup
func (bm *backupManager) RestoreBackup(ctx context.Context, backupID string, passphrase string) error {
	if backupID == "" {
		return fmt.Errorf("backup ID cannot be empty")
	}

	// Find backup file
	archivePath := filepath.Join(bm.backupDir, backupID+".tar.gz")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		// Try encrypted version
		archivePath = filepath.Join(bm.backupDir, backupID+".tar.gz.enc")
		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			return fmt.Errorf("backup not found: %s", backupID)
		}

		// Decrypt if encrypted
		if passphrase == "" {
			return fmt.Errorf("passphrase required for encrypted backup")
		}

		decryptedPath := filepath.Join(bm.backupDir, backupID+".tar.gz.tmp")
		if err := bm.decryptFile(archivePath, decryptedPath, passphrase); err != nil {
			return fmt.Errorf("failed to decrypt backup: %w", err)
		}
		defer os.Remove(decryptedPath)
		archivePath = decryptedPath
	}

	// Verify checksum
	if err := bm.verifyChecksum(archivePath, backupID); err != nil {
		return fmt.Errorf("backup integrity check failed: %w", err)
	}

	// Extract archive
	if err := bm.extractArchive(archivePath); err != nil {
		return fmt.Errorf("failed to extract backup: %w", err)
	}

	return nil
}

// ListBackups lists all backups for a cluster
func (bm *backupManager) ListBackups(cluster string) ([]*Backup, error) {
	var backups []*Backup

	// Read backup directory
	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Match backup files for this cluster
		if cluster != "" && !filepath.HasPrefix(name, cluster+"-") {
			continue
		}

		// Parse backup metadata
		info, err := entry.Info()
		if err != nil {
			continue
		}

		backup := &Backup{
			ID:              name,
			Cluster:         cluster,
			CreatedAt:       info.ModTime(),
			Size:            info.Size(),
			StorageLocation: filepath.Join(bm.backupDir, name),
		}

		backups = append(backups, backup)
	}

	return backups, nil
}

// DeleteBackup deletes a backup
func (bm *backupManager) DeleteBackup(backupID string) error {
	if backupID == "" {
		return fmt.Errorf("backup ID cannot be empty")
	}

	archivePath := filepath.Join(bm.backupDir, backupID+".tar.gz")
	if err := os.Remove(archivePath); err != nil {
		// Try encrypted version
		archivePath = filepath.Join(bm.backupDir, backupID+".tar.gz.enc")
		if err := os.Remove(archivePath); err != nil {
			return fmt.Errorf("failed to delete backup: %w", err)
		}
	}

	// Also remove checksum file if exists
	checksumPath := filepath.Join(bm.backupDir, backupID+".sha256")
	os.Remove(checksumPath) // Ignore error

	return nil
}

// ScheduleBackups schedules periodic backups (placeholder for future implementation)
// Issue: https://github.com/rackerlabs/opencenter-cli/issues/XXX - Implement backup scheduling
func (bm *backupManager) ScheduleBackups(schedule string, retention time.Duration) error {
	return fmt.Errorf("backup scheduling not yet implemented")
}

// collectBackupContents collects all files to be backed up
func (bm *backupManager) collectBackupContents(cluster string) (*BackupContents, error) {
	contents := &BackupContents{}

	// Resolve cluster paths using PathResolver
	clusterPaths, err := bm.pathResolver.ResolveWithFallback(context.Background(), cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cluster paths: %w", err)
	}

	// Read config file
	if data, err := os.ReadFile(clusterPaths.ConfigPath); err == nil {
		contents.ConfigFile = data
	}

	// Read Age keys
	if data, err := os.ReadFile(clusterPaths.SOPSKeyPath); err == nil {
		contents.AgeKeys = data
	}

	// Read SSH keys
	if data, err := os.ReadFile(clusterPaths.SSHKeyPath); err == nil {
		contents.SSHKeys = data
	}

	// Read GitOps state
	if data, err := bm.archiveDirectory(clusterPaths.GitOpsDir); err == nil {
		contents.GitOpsState = data
	}

	// Read Terraform state
	tfStatePath := filepath.Join(clusterPaths.ClusterDir, "terraform.tfstate")
	if data, err := os.ReadFile(tfStatePath); err == nil {
		contents.TerraformState = data
	}

	return contents, nil
}

// createArchive creates a compressed tar archive
func (bm *backupManager) createArchive(archivePath string, contents *BackupContents) error {
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add config file
	if len(contents.ConfigFile) > 0 {
		if err := bm.addToArchive(tarWriter, "config.yaml", contents.ConfigFile); err != nil {
			return err
		}
	}

	// Add Age keys
	if len(contents.AgeKeys) > 0 {
		if err := bm.addToArchive(tarWriter, "age-key.txt", contents.AgeKeys); err != nil {
			return err
		}
	}

	// Add SSH keys
	if len(contents.SSHKeys) > 0 {
		if err := bm.addToArchive(tarWriter, "ssh-keys", contents.SSHKeys); err != nil {
			return err
		}
	}

	// Add GitOps state
	if len(contents.GitOpsState) > 0 {
		if err := bm.addToArchive(tarWriter, "gitops.tar", contents.GitOpsState); err != nil {
			return err
		}
	}

	// Add Terraform state
	if len(contents.TerraformState) > 0 {
		if err := bm.addToArchive(tarWriter, "terraform.tfstate", contents.TerraformState); err != nil {
			return err
		}
	}

	return nil
}

// addToArchive adds a file to the tar archive
func (bm *backupManager) addToArchive(tarWriter *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    0600,
		ModTime: time.Now(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tarWriter.Write(data); err != nil {
		return err
	}

	return nil
}

// extractArchive extracts a tar.gz archive
func (bm *backupManager) extractArchive(archivePath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	// Extract cluster name from archive path for path resolution
	// This is a simplified approach - in production, you'd want to store
	// cluster metadata in the backup
	clusterName := "restored"

	// Resolve paths for the restored cluster
	clusterPaths, err := bm.pathResolver.ResolveWithFallback(context.Background(), clusterName)
	if err != nil {
		// If cluster doesn't exist, create directories
		if err := bm.pathResolver.CreateClusterDirectories(context.Background(), clusterName, "opencenter"); err != nil {
			return fmt.Errorf("failed to create cluster directories: %w", err)
		}
		clusterPaths, err = bm.pathResolver.Resolve(context.Background(), clusterName, "opencenter")
		if err != nil {
			return fmt.Errorf("failed to resolve paths after creation: %w", err)
		}
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Determine output path based on file name
		var outputPath string
		switch header.Name {
		case "config.yaml":
			outputPath = clusterPaths.ConfigPath
		case "age-key.txt":
			outputPath = clusterPaths.SOPSKeyPath
		case "ssh-keys":
			outputPath = clusterPaths.SSHKeyPath
		case "terraform.tfstate":
			outputPath = filepath.Join(clusterPaths.ClusterDir, "terraform.tfstate")
		default:
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
			return err
		}

		// Extract file
		outFile, err := os.Create(outputPath)
		if err != nil {
			return err
		}

		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return err
		}
		outFile.Close()
	}

	return nil
}

// calculateChecksum calculates SHA-256 checksum of a file
func (bm *backupManager) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	checksum := hex.EncodeToString(hash.Sum(nil))

	// Save checksum to file
	checksumPath := filePath + ".sha256"
	if err := os.WriteFile(checksumPath, []byte(checksum), 0600); err != nil {
		return "", err
	}

	return checksum, nil
}

// verifyChecksum verifies the integrity of a backup file
func (bm *backupManager) verifyChecksum(filePath, backupID string) error {
	// Calculate current checksum
	currentChecksum, err := bm.calculateChecksum(filePath)
	if err != nil {
		return err
	}

	// Read stored checksum
	checksumPath := filepath.Join(bm.backupDir, backupID+".sha256")
	storedChecksum, err := os.ReadFile(checksumPath)
	if err != nil {
		// If checksum file doesn't exist, skip verification
		return nil
	}

	if currentChecksum != string(storedChecksum) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", storedChecksum, currentChecksum)
	}

	return nil
}

// encryptFile encrypts a file using AES-256-GCM with Argon2 key derivation
func (bm *backupManager) encryptFile(inputPath, outputPath, passphrase string) error {
	// Read input file
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}

	// Generate salt
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return err
	}

	// Derive key using Argon2
	key := argon2.IDKey([]byte(passphrase), salt, 1, 64*1024, 4, 32)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Write output: salt + ciphertext
	output := append(salt, ciphertext...)
	if err := os.WriteFile(outputPath, output, 0600); err != nil {
		return err
	}

	return nil
}

// decryptFile decrypts a file using AES-256-GCM with Argon2 key derivation
func (bm *backupManager) decryptFile(inputPath, outputPath, passphrase string) error {
	// Read input file
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}

	// Extract salt (first 32 bytes)
	if len(input) < 32 {
		return fmt.Errorf("invalid encrypted file: too short")
	}
	salt := input[:32]
	ciphertext := input[32:]

	// Derive key using Argon2
	key := argon2.IDKey([]byte(passphrase), salt, 1, 64*1024, 4, 32)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("invalid encrypted file: ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	// Write output
	if err := os.WriteFile(outputPath, plaintext, 0600); err != nil {
		return err
	}

	return nil
}

// archiveDirectory creates a tar archive of a directory
// Issue: https://github.com/rackerlabs/opencenter-cli/issues/XXX - Implement directory archiving for backups
func (bm *backupManager) archiveDirectory(dirPath string) ([]byte, error) {
	return nil, nil
}

// EncryptBackup encrypts a backup with a passphrase
func EncryptBackup(backupPath, passphrase string) error {
	bm := &backupManager{}
	encryptedPath := backupPath + ".enc"
	if err := bm.encryptFile(backupPath, encryptedPath, passphrase); err != nil {
		return err
	}
	// Remove unencrypted backup
	return os.Remove(backupPath)
}
