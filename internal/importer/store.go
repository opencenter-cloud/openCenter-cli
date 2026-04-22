package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
)

const artifactSubdir = "importer/scans"

type ArtifactStore struct {
	root string
}

type storedArtifact struct {
	RepoHash string           `json:"repo_hash"`
	SavedAt  time.Time        `json:"saved_at"`
	Result   ImportScanResult `json:"result"`
}

func NewArtifactStore() (*ArtifactStore, error) {
	root, err := config.ResolveStateDir()
	if err != nil {
		return nil, fmt.Errorf("resolve state dir: %w", err)
	}

	return &ArtifactStore{
		root: filepath.Join(root, artifactSubdir),
	}, nil
}

func RepoHash(repoPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(repoPath)))
	return hex.EncodeToString(sum[:8])
}

func (s *ArtifactStore) Save(repoPath string, result *ImportScanResult, savedAt time.Time) (*SavedArtifact, error) {
	if result == nil {
		return nil, fmt.Errorf("scan result cannot be nil")
	}
	if savedAt.IsZero() {
		savedAt = time.Now().UTC()
	}

	repoHash := RepoHash(repoPath)
	dir := filepath.Join(s.root, repoHash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact dir: %w", err)
	}

	resultCopy := *result
	resultCopy.RepoPath = repoPath
	resultCopy.ScannedAt = savedAt

	payload := storedArtifact{
		RepoHash: repoHash,
		SavedAt:  savedAt,
		Result:   resultCopy,
	}

	filename := savedAt.UTC().Format("20060102T150405Z") + ".json"
	path := filepath.Join(dir, filename)
	if err := writeJSON(path, payload); err != nil {
		return nil, err
	}

	latestPath := filepath.Join(dir, "latest.json")
	if err := writeJSON(latestPath, payload); err != nil {
		return nil, err
	}

	return &SavedArtifact{
		RepoHash: repoHash,
		Path:     latestPath,
		SavedAt:  savedAt,
		Result:   &resultCopy,
	}, nil
}

func (s *ArtifactStore) LoadLatest(repoPath string) (*SavedArtifact, error) {
	repoHash := RepoHash(repoPath)
	path := filepath.Join(s.root, repoHash, "latest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read latest artifact: %w", err)
	}

	var payload storedArtifact
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode latest artifact: %w", err)
	}

	return &SavedArtifact{
		RepoHash: repoHash,
		Path:     path,
		SavedAt:  payload.SavedAt,
		Result:   &payload.Result,
	}, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}

	return nil
}
