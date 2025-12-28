package objectstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sync"
	"time"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/dashboardsnapshots"
)

// snapshotMetadata is the metadata stored in object storage
type snapshotMetadata struct {
	ID                int64     `json:"id"`
	Key               string    `json:"key"`
	DeleteKey         string    `json:"delete_key"`
	Name              string    `json:"name"`
	OrgID             int64     `json:"org_id"`
	UserID            int64     `json:"user_id"`
	External          bool      `json:"external"`
	ExternalURL       string    `json:"external_url"`
	ExternalDeleteURL string    `json:"external_delete_url"`
	Expires           time.Time `json:"expires"`
	Created           time.Time `json:"created"`
	Updated           time.Time `json:"updated"`
}

// snapshotData contains the encrypted dashboard data
type snapshotData struct {
	DashboardEncrypted []byte `json:"dashboard_encrypted"`
}

// orgIndex holds the list of snapshot keys for an organization
type orgIndex struct {
	Keys []string `json:"keys"`
}

// ObjectStorageStore implements dashboardsnapshots.Store using object storage
type ObjectStorageStore struct {
	storage    ObjectStorage
	pathPrefix string
	log        log.Logger
	idCounter  int64
	idMu       sync.Mutex
}

// Ensure ObjectStorageStore implements Store interface
var _ dashboardsnapshots.Store = (*ObjectStorageStore)(nil)

// NewObjectStorageStore creates a new object storage based store
func NewObjectStorageStore(storage ObjectStorage, cfg Config) *ObjectStorageStore {
	return &ObjectStorageStore{
		storage:    storage,
		pathPrefix: cfg.PathPrefix,
		log:        log.New("dashboardsnapshot.objectstore"),
		idCounter:  time.Now().UnixNano(), // Use timestamp as base for IDs
	}
}

func (s *ObjectStorageStore) metaPath(key string) string {
	return path.Join(s.pathPrefix, "meta", key+".json")
}

func (s *ObjectStorageStore) dataPath(key string) string {
	return path.Join(s.pathPrefix, "data", key+".json")
}

func (s *ObjectStorageStore) indexPath(orgID int64) string {
	return path.Join(s.pathPrefix, "index", fmt.Sprintf("org_%d.json", orgID))
}

func (s *ObjectStorageStore) deleteKeyIndexPath() string {
	return path.Join(s.pathPrefix, "index", "delete_keys.json")
}

// generateID generates a unique ID for a snapshot
func (s *ObjectStorageStore) generateID() int64 {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	s.idCounter++
	return s.idCounter
}

// CreateDashboardSnapshot creates a new snapshot in object storage
func (s *ObjectStorageStore) CreateDashboardSnapshot(ctx context.Context, cmd *dashboardsnapshots.CreateDashboardSnapshotCommand) (*dashboardsnapshots.DashboardSnapshot, error) {
	var expires = time.Now().Add(time.Hour * 24 * 365 * 50)
	if cmd.Expires > 0 {
		expires = time.Now().Add(time.Second * time.Duration(cmd.Expires))
	}

	now := time.Now()
	id := s.generateID()

	// Create metadata
	meta := snapshotMetadata{
		ID:                id,
		Key:               cmd.Key,
		DeleteKey:         cmd.DeleteKey,
		Name:              cmd.Name,
		OrgID:             cmd.OrgID,
		UserID:            cmd.UserID,
		External:          cmd.External,
		ExternalURL:       cmd.ExternalURL,
		ExternalDeleteURL: cmd.ExternalDeleteURL,
		Expires:           expires,
		Created:           now,
		Updated:           now,
	}

	// Create data
	data := snapshotData{
		DashboardEncrypted: cmd.DashboardEncrypted,
	}

	// Store metadata
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := s.storage.Put(ctx, s.metaPath(cmd.Key), bytes.NewReader(metaBytes), int64(len(metaBytes))); err != nil {
		return nil, fmt.Errorf("failed to store metadata: %w", err)
	}

	// Store data
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}
	if err := s.storage.Put(ctx, s.dataPath(cmd.Key), bytes.NewReader(dataBytes), int64(len(dataBytes))); err != nil {
		return nil, fmt.Errorf("failed to store data: %w", err)
	}

	// Update org index
	if err := s.addToOrgIndex(ctx, cmd.OrgID, cmd.Key); err != nil {
		s.log.Warn("failed to update org index", "error", err)
	}

	// Update delete key index
	if err := s.addToDeleteKeyIndex(ctx, cmd.DeleteKey, cmd.Key); err != nil {
		s.log.Warn("failed to update delete key index", "error", err)
	}

	return &dashboardsnapshots.DashboardSnapshot{
		ID:                 id,
		Name:               cmd.Name,
		Key:                cmd.Key,
		DeleteKey:          cmd.DeleteKey,
		OrgID:              cmd.OrgID,
		UserID:             cmd.UserID,
		External:           cmd.External,
		ExternalURL:        cmd.ExternalURL,
		ExternalDeleteURL:  cmd.ExternalDeleteURL,
		Expires:            expires,
		Created:            now,
		Updated:            now,
		DashboardEncrypted: cmd.DashboardEncrypted,
	}, nil
}

// addToOrgIndex adds a snapshot key to the org's index
func (s *ObjectStorageStore) addToOrgIndex(ctx context.Context, orgID int64, key string) error {
	indexPath := s.indexPath(orgID)

	var index orgIndex
	if reader, err := s.storage.Get(ctx, indexPath); err == nil {
		defer reader.Close()
		data, _ := io.ReadAll(reader)
		json.Unmarshal(data, &index)
	}

	// Check if key already exists
	for _, k := range index.Keys {
		if k == key {
			return nil
		}
	}

	index.Keys = append(index.Keys, key)

	indexBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}

	return s.storage.Put(ctx, indexPath, bytes.NewReader(indexBytes), int64(len(indexBytes)))
}

// deleteKeyIndex maps delete_key -> key
type deleteKeyIndex struct {
	Mapping map[string]string `json:"mapping"`
}

// addToDeleteKeyIndex adds a delete_key -> key mapping
func (s *ObjectStorageStore) addToDeleteKeyIndex(ctx context.Context, deleteKey, key string) error {
	indexPath := s.deleteKeyIndexPath()

	var index deleteKeyIndex
	index.Mapping = make(map[string]string)

	if reader, err := s.storage.Get(ctx, indexPath); err == nil {
		defer reader.Close()
		data, _ := io.ReadAll(reader)
		json.Unmarshal(data, &index)
		if index.Mapping == nil {
			index.Mapping = make(map[string]string)
		}
	}

	index.Mapping[deleteKey] = key

	indexBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}

	return s.storage.Put(ctx, indexPath, bytes.NewReader(indexBytes), int64(len(indexBytes)))
}

// GetDashboardSnapshot retrieves a snapshot by key or delete_key
func (s *ObjectStorageStore) GetDashboardSnapshot(ctx context.Context, query *dashboardsnapshots.GetDashboardSnapshotQuery) (*dashboardsnapshots.DashboardSnapshot, error) {
	key := query.Key

	// If querying by DeleteKey, lookup the actual key first
	if key == "" && query.DeleteKey != "" {
		var err error
		key, err = s.lookupKeyByDeleteKey(ctx, query.DeleteKey)
		if err != nil {
			return nil, err
		}
	}

	if key == "" {
		return nil, dashboardsnapshots.ErrBaseNotFound.Errorf("dashboard snapshot not found")
	}

	// Get metadata
	metaReader, err := s.storage.Get(ctx, s.metaPath(key))
	if err != nil {
		return nil, dashboardsnapshots.ErrBaseNotFound.Errorf("dashboard snapshot not found")
	}
	defer metaReader.Close()

	metaBytes, err := io.ReadAll(metaReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var meta snapshotMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Get data
	dataReader, err := s.storage.Get(ctx, s.dataPath(key))
	if err != nil {
		return nil, fmt.Errorf("failed to get data: %w", err)
	}
	defer dataReader.Close()

	dataBytes, err := io.ReadAll(dataReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	var data snapshotData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return &dashboardsnapshots.DashboardSnapshot{
		ID:                 meta.ID,
		Name:               meta.Name,
		Key:                meta.Key,
		DeleteKey:          meta.DeleteKey,
		OrgID:              meta.OrgID,
		UserID:             meta.UserID,
		External:           meta.External,
		ExternalURL:        meta.ExternalURL,
		ExternalDeleteURL:  meta.ExternalDeleteURL,
		Expires:            meta.Expires,
		Created:            meta.Created,
		Updated:            meta.Updated,
		DashboardEncrypted: data.DashboardEncrypted,
	}, nil
}

// lookupKeyByDeleteKey finds the snapshot key by delete key
func (s *ObjectStorageStore) lookupKeyByDeleteKey(ctx context.Context, deleteKey string) (string, error) {
	indexPath := s.deleteKeyIndexPath()

	reader, err := s.storage.Get(ctx, indexPath)
	if err != nil {
		return "", dashboardsnapshots.ErrBaseNotFound.Errorf("dashboard snapshot not found")
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	var index deleteKeyIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return "", err
	}

	key, ok := index.Mapping[deleteKey]
	if !ok {
		return "", dashboardsnapshots.ErrBaseNotFound.Errorf("dashboard snapshot not found")
	}

	return key, nil
}

// SearchDashboardSnapshots returns snapshots for an organization
func (s *ObjectStorageStore) SearchDashboardSnapshots(ctx context.Context, query *dashboardsnapshots.GetDashboardSnapshotsQuery) (dashboardsnapshots.DashboardSnapshotsList, error) {
	indexPath := s.indexPath(query.OrgID)

	reader, err := s.storage.Get(ctx, indexPath)
	if err != nil {
		// No index means no snapshots
		return dashboardsnapshots.DashboardSnapshotsList{}, nil
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	var index orgIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index: %w", err)
	}

	var results dashboardsnapshots.DashboardSnapshotsList
	limit := query.Limit
	if limit <= 0 {
		limit = 100 // Default limit
	}

	for _, key := range index.Keys {
		if len(results) >= limit {
			break
		}

		metaReader, err := s.storage.Get(ctx, s.metaPath(key))
		if err != nil {
			continue // Skip if metadata not found
		}

		metaBytes, err := io.ReadAll(metaReader)
		metaReader.Close()
		if err != nil {
			continue
		}

		var meta snapshotMetadata
		if err := json.Unmarshal(metaBytes, &meta); err != nil {
			continue
		}

		// Filter by name if provided
		if query.Name != "" {
			// Simple contains match (original uses LIKE)
			if !containsIgnoreCase(meta.Name, query.Name) {
				continue
			}
		}

		// Check user permissions (non-admin can only see their own)
		if query.SignedInUser != nil && query.SignedInUser.OrgRole != "Admin" && !query.SignedInUser.IsAnonymous {
			if meta.UserID != query.SignedInUser.UserID {
				continue
			}
		}

		results = append(results, &dashboardsnapshots.DashboardSnapshotDTO{
			ID:          meta.ID,
			Name:        meta.Name,
			Key:         meta.Key,
			OrgID:       meta.OrgID,
			UserID:      meta.UserID,
			External:    meta.External,
			ExternalURL: meta.ExternalURL,
			Expires:     meta.Expires,
			Created:     meta.Created,
			Updated:     meta.Updated,
		})
	}

	return results, nil
}

// DeleteDashboardSnapshot deletes a snapshot by delete key
func (s *ObjectStorageStore) DeleteDashboardSnapshot(ctx context.Context, cmd *dashboardsnapshots.DeleteDashboardSnapshotCommand) error {
	// Find the key by delete key
	key, err := s.lookupKeyByDeleteKey(ctx, cmd.DeleteKey)
	if err != nil {
		return nil // Not found is not an error for delete
	}

	// Get metadata to find orgID for index cleanup
	metaReader, err := s.storage.Get(ctx, s.metaPath(key))
	var orgID int64
	if err == nil {
		metaBytes, _ := io.ReadAll(metaReader)
		metaReader.Close()
		var meta snapshotMetadata
		if json.Unmarshal(metaBytes, &meta) == nil {
			orgID = meta.OrgID
		}
	}

	// Delete data and metadata
	s.storage.Delete(ctx, s.dataPath(key))
	s.storage.Delete(ctx, s.metaPath(key))

	// Update indexes
	if orgID > 0 {
		s.removeFromOrgIndex(ctx, orgID, key)
	}
	s.removeFromDeleteKeyIndex(ctx, cmd.DeleteKey)

	return nil
}

// removeFromOrgIndex removes a key from the org index
func (s *ObjectStorageStore) removeFromOrgIndex(ctx context.Context, orgID int64, key string) error {
	indexPath := s.indexPath(orgID)

	reader, err := s.storage.Get(ctx, indexPath)
	if err != nil {
		return nil
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	var index orgIndex
	json.Unmarshal(data, &index)

	// Remove key
	var newKeys []string
	for _, k := range index.Keys {
		if k != key {
			newKeys = append(newKeys, k)
		}
	}
	index.Keys = newKeys

	indexBytes, _ := json.Marshal(index)
	return s.storage.Put(ctx, indexPath, bytes.NewReader(indexBytes), int64(len(indexBytes)))
}

// removeFromDeleteKeyIndex removes a delete key mapping
func (s *ObjectStorageStore) removeFromDeleteKeyIndex(ctx context.Context, deleteKey string) error {
	indexPath := s.deleteKeyIndexPath()

	reader, err := s.storage.Get(ctx, indexPath)
	if err != nil {
		return nil
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	var index deleteKeyIndex
	json.Unmarshal(data, &index)

	delete(index.Mapping, deleteKey)

	indexBytes, _ := json.Marshal(index)
	return s.storage.Put(ctx, indexPath, bytes.NewReader(indexBytes), int64(len(indexBytes)))
}

// DeleteExpiredSnapshots removes expired snapshots
func (s *ObjectStorageStore) DeleteExpiredSnapshots(ctx context.Context, cmd *dashboardsnapshots.DeleteExpiredSnapshotsCommand) error {
	// List all meta files
	keys, err := s.storage.List(ctx, path.Join(s.pathPrefix, "meta"))
	if err != nil {
		return err
	}

	now := time.Now()
	var deletedCount int64

	for _, metaPath := range keys {
		reader, err := s.storage.Get(ctx, metaPath)
		if err != nil {
			continue
		}

		data, _ := io.ReadAll(reader)
		reader.Close()

		var meta snapshotMetadata
		if json.Unmarshal(data, &meta) != nil {
			continue
		}

		// Check if expired
		if meta.Expires.Before(now) {
			// Delete this snapshot
			if err := s.DeleteDashboardSnapshot(ctx, &dashboardsnapshots.DeleteDashboardSnapshotCommand{
				DeleteKey: meta.DeleteKey,
			}); err == nil {
				deletedCount++
			}
		}
	}

	cmd.DeletedRows = deletedCount
	return nil
}

// containsIgnoreCase checks if s contains substr (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" || findIgnoreCase(s, substr))
}

func findIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if matchIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func matchIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
