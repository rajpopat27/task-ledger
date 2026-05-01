package file

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"task-ledger/internal/storage"
	"task-ledger/internal/types"
)

const (
	issueFileName   = "issue.json"
	projectDirMode  = 0o750
	projectFileMode = 0o644
	lockFileMode    = 0o600
)

// FileStorage stores issues as recursive .task-ledger/issues/**/issue.json files.
type FileStorage struct {
	*issueIndex

	taskLedgerDir string
	issuesDir     string
	pathByID      map[string]string
	issuesByID    map[string]*types.Issue
	lockTimeout   time.Duration
	lockDepth     int
}

// Open loads a file-backed Task Ledger repository from taskLedgerDir.
func Open(taskLedgerDir string) (*FileStorage, error) {
	if taskLedgerDir == "" {
		return nil, fmt.Errorf("taskLedgerDir is required")
	}
	store := &FileStorage{
		issueIndex:    newIssueIndex(),
		taskLedgerDir: taskLedgerDir,
		issuesDir:     filepath.Join(taskLedgerDir, "issues"),
		pathByID:      make(map[string]string),
		issuesByID:    make(map[string]*types.Issue),
		lockTimeout:   5 * time.Second,
	}
	if err := os.MkdirAll(store.issuesDir, projectDirMode); err != nil {
		return nil, fmt.Errorf("create issues dir: %w", err)
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileStorage) load() error {
	files, err := FindIssueFiles(s.taskLedgerDir)
	if err != nil {
		return err
	}
	issues := make([]*types.Issue, 0, len(files))
	for _, path := range files {
		issue, err := readIssueFile(path)
		if err != nil {
			return err
		}
		if existingPath, exists := s.pathByID[issue.ID]; exists {
			return fmt.Errorf("duplicate issue id %s in %s and %s", issue.ID, existingPath, path)
		}
		rel, err := filepath.Rel(s.taskLedgerDir, path)
		if err != nil {
			return err
		}
		s.pathByID[issue.ID] = rel
		s.issuesByID[issue.ID] = issue
		issues = append(issues, issue)
	}
	if err := s.issueIndex.LoadFromIssues(issues); err != nil {
		return err
	}
	return nil
}

// FindIssueFiles returns every issue.json below .task-ledger/issues.
func FindIssueFiles(taskLedgerDir string) ([]string, error) {
	issuesDir := filepath.Join(taskLedgerDir, "issues")
	files := make([]string, 0)
	if _, err := os.Stat(issuesDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return files, nil
		}
		return nil, err
	}
	err := filepath.WalkDir(issuesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == issueFileName {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func readIssueFile(path string) (*types.Issue, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from FindIssueFiles under the configured .task-ledger directory.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var fileIssue FileIssue
	if err := json.Unmarshal(data, &fileIssue); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	issue, err := fileIssue.ToTypesIssue()
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return issue, nil
}

// Path returns the repository storage path.
func (s *FileStorage) Path() string {
	return s.taskLedgerDir
}

func (s *FileStorage) CreateIssue(ctx context.Context, issue *types.Issue, actor string) error {
	return s.withWriteLock(func() error {
		if issue.IssueType == "" {
			issue.IssueType = types.TypeTask
		}
		if issue.Status == "" {
			issue.Status = types.StatusOpen
		}
		if issue.ID == "" {
			id, err := s.generateID(issue.IssueType)
			if err != nil {
				return err
			}
			issue.ID = id
		}
		if err := ValidateFileIssueID(issue.ID); err != nil {
			return err
		}
		if err := s.issueIndex.CreateIssue(ctx, issue, actor); err != nil {
			return err
		}
		return s.saveIssueID(ctx, issue.ID)
	})
}

func (s *FileStorage) CreateIssues(ctx context.Context, issues []*types.Issue, actor string) error {
	return s.withWriteLock(func() error {
		snapshot, err := s.snapshotIssueFiles()
		if err != nil {
			return err
		}

		if err := s.prepareIssuesForCreate(issues); err != nil {
			return err
		}
		if err := s.issueIndex.CreateIssues(ctx, issues, actor); err != nil {
			return s.rollbackSnapshot(snapshot, err)
		}
		for _, issue := range issues {
			if err := s.saveIssueID(ctx, issue.ID); err != nil {
				return s.rollbackSnapshot(snapshot, err)
			}
		}
		return nil
	})
}

// ImportIssue writes an existing issue snapshot, including labels, dependencies,
// and comments. It is intended for migration from Task Ledger JSONL snapshots.
func (s *FileStorage) ImportIssue(ctx context.Context, issue *types.Issue, actor string) error {
	return s.ImportIssues(ctx, []*types.Issue{issue}, actor)
}

// ImportIssues writes existing issue snapshots as one all-or-nothing batch.
func (s *FileStorage) ImportIssues(ctx context.Context, issues []*types.Issue, actor string) error {
	return s.withWriteLock(func() error {
		snapshot, err := s.snapshotIssueFiles()
		if err != nil {
			return err
		}

		if err := s.prepareIssuesForImport(issues, actor); err != nil {
			return err
		}
		if err := s.issueIndex.LoadFromIssues(issues); err != nil {
			return s.rollbackSnapshot(snapshot, err)
		}
		for _, issue := range issues {
			if err := s.saveIssueID(ctx, issue.ID); err != nil {
				return s.rollbackSnapshot(snapshot, err)
			}
		}

		childIDs := make(map[string]struct{})
		importedIDs := make(map[string]struct{}, len(issues))
		for _, issue := range issues {
			importedIDs[issue.ID] = struct{}{}
		}
		for _, issue := range issues {
			for childID, child := range s.issuesByID {
				if childID != issue.ID && parentID(child) == issue.ID {
					if _, imported := importedIDs[childID]; !imported {
						childIDs[childID] = struct{}{}
					}
				}
			}
		}
		for childID := range childIDs {
			if err := s.saveIssueID(ctx, childID); err != nil {
				return s.rollbackSnapshot(snapshot, err)
			}
		}
		return nil
	})
}

func (s *FileStorage) UpdateIssue(ctx context.Context, id string, updates map[string]interface{}, actor string) error {
	return s.withWriteLock(func() error {
		if err := s.issueIndex.UpdateIssue(ctx, id, updates, actor); err != nil {
			return err
		}
		return s.saveIssueID(ctx, id)
	})
}

func (s *FileStorage) CloseIssue(ctx context.Context, id string, reason string, actor string, session string) error {
	return s.withWriteLock(func() error {
		if err := s.issueIndex.CloseIssue(ctx, id, reason, actor, session); err != nil {
			return err
		}
		return s.saveIssueID(ctx, id)
	})
}

func (s *FileStorage) DeleteIssue(ctx context.Context, id string) error {
	return s.withWriteLock(func() error {
		oldPath := s.absolutePathForID(id)
		if err := s.issueIndex.DeleteIssue(ctx, id); err != nil {
			return err
		}
		delete(s.pathByID, id)
		delete(s.issuesByID, id)
		if oldPath != "" {
			if err := os.Remove(oldPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			pruneEmptyDirs(filepath.Dir(oldPath), s.issuesDir)
		}
		return nil
	})
}

func (s *FileStorage) DeleteIssueWithReason(ctx context.Context, id, actor, reason string) error {
	return s.withWriteLock(func() error {
		if err := s.issueIndex.CreateTombstone(ctx, id, actor, reason); err != nil {
			return err
		}
		return s.saveIssueID(ctx, id)
	})
}

func (s *FileStorage) AddDependency(ctx context.Context, dep *types.Dependency, actor string) error {
	return s.withWriteLock(func() error {
		if dep.CreatedAt.IsZero() {
			dep.CreatedAt = time.Now()
		}
		if dep.CreatedBy == "" {
			dep.CreatedBy = actor
		}
		if err := s.issueIndex.AddDependency(ctx, dep, actor); err != nil {
			return err
		}
		return s.saveIssueID(ctx, dep.IssueID)
	})
}

func (s *FileStorage) RemoveDependency(ctx context.Context, issueID, dependsOnID string, actor string) error {
	return s.withWriteLock(func() error {
		if err := s.issueIndex.RemoveDependency(ctx, issueID, dependsOnID, actor); err != nil {
			return err
		}
		return s.saveIssueID(ctx, issueID)
	})
}

func (s *FileStorage) AddLabel(ctx context.Context, issueID, label, actor string) error {
	return s.withWriteLock(func() error {
		if err := s.issueIndex.AddLabel(ctx, issueID, label, actor); err != nil {
			return err
		}
		return s.saveIssueID(ctx, issueID)
	})
}

func (s *FileStorage) RemoveLabel(ctx context.Context, issueID, label, actor string) error {
	return s.withWriteLock(func() error {
		if err := s.issueIndex.RemoveLabel(ctx, issueID, label, actor); err != nil {
			return err
		}
		return s.saveIssueID(ctx, issueID)
	})
}

func (s *FileStorage) AddIssueComment(ctx context.Context, issueID, author, text string) (*types.Comment, error) {
	var comment *types.Comment
	err := s.withWriteLock(func() error {
		var err error
		comment, err = s.issueIndex.AddIssueComment(ctx, issueID, author, text)
		if err != nil {
			return err
		}
		return s.saveIssueID(ctx, issueID)
	})
	return comment, err
}

func (s *FileStorage) AddComment(ctx context.Context, issueID, actor, comment string) error {
	_, err := s.AddIssueComment(ctx, issueID, actor, comment)
	return err
}

func (s *FileStorage) RunInTransaction(ctx context.Context, fn func(tx storage.Transaction) error) error {
	return s.withWriteLock(func() error {
		snapshot, err := s.snapshotIssueFiles()
		if err != nil {
			return err
		}
		if err := fn(s); err != nil {
			return s.rollbackSnapshot(snapshot, err)
		}
		return nil
	})
}

func (s *FileStorage) GetNextChildID(ctx context.Context, parentID string) (string, error) {
	parent, err := s.GetIssue(ctx, parentID)
	if err != nil {
		return "", err
	}
	if parent == nil {
		return "", fmt.Errorf("parent issue %s not found", parentID)
	}
	childType := types.TypeTask
	if parent.IssueType == types.TypeFeature || parent.IssueType == types.TypeFeatureRequest {
		childType = types.TypeEpic
	}
	return s.generateID(childType)
}

func (s *FileStorage) saveIssueID(ctx context.Context, id string) error {
	if err := ValidateFileIssueID(id); err != nil {
		return err
	}
	issue, err := s.issueForPersistence(ctx, id)
	if err != nil {
		return err
	}
	if issue == nil {
		return fmt.Errorf("issue %s not found", id)
	}
	oldRel := s.pathByID[id]
	newRel := s.pathForIssue(issue)
	newAbs := filepath.Join(s.taskLedgerDir, newRel)
	if err := os.MkdirAll(filepath.Dir(newAbs), projectDirMode); err != nil {
		return err
	}
	if err := writeFileIssueAtomic(newAbs, s.fileIssueFor(issue)); err != nil {
		return err
	}
	if oldRel != "" && oldRel != newRel {
		oldAbs := filepath.Join(s.taskLedgerDir, oldRel)
		if err := os.Remove(oldAbs); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		pruneEmptyDirs(filepath.Dir(oldAbs), s.issuesDir)
	}
	s.pathByID[id] = newRel
	issueCopy := *issue
	s.issuesByID[id] = &issueCopy
	return nil
}

func (s *FileStorage) issueForPersistence(ctx context.Context, id string) (*types.Issue, error) {
	issue, err := s.GetIssue(ctx, id)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, nil
	}
	comments, err := s.GetIssueComments(ctx, id)
	if err != nil {
		return nil, err
	}
	issue.Comments = append([]*types.Comment(nil), comments...)
	return issue, nil
}

func (s *FileStorage) fileIssueFor(issue *types.Issue) FileIssue {
	fileIssue := FromTypesIssue(issue)
	parentID := parentID(issue)
	if parentID == "" {
		return fileIssue
	}
	parentIssue := s.issuesByID[parentID]
	if parentIssue == nil {
		return fileIssue
	}
	switch parentIssue.IssueType {
	case types.TypeFeature, types.TypeFeatureRequest:
		fileIssue.FeatureRequest = parentID
	case types.TypeEpic:
		fileIssue.Epic = parentID
		grandparentID := parentIDOfStored(parentIssue)
		if grandparentID != "" {
			if grandparent := s.issuesByID[grandparentID]; grandparent != nil &&
				(grandparent.IssueType == types.TypeFeature || grandparent.IssueType == types.TypeFeatureRequest) {
				fileIssue.FeatureRequest = grandparentID
			}
		}
	}
	return fileIssue
}

func parentIDOfStored(issue *types.Issue) string {
	return parentID(issue)
}

func (s *FileStorage) pathForIssue(issue *types.Issue) string {
	parentID := parentID(issue)
	if parentID == "" {
		return filepath.Join("issues", issue.ID, issueFileName)
	}
	parentRel, ok := s.pathByID[parentID]
	if !ok {
		return filepath.Join("issues", issue.ID, issueFileName)
	}
	parentDir := filepath.Dir(parentRel)
	parentIssue := s.issuesByID[parentID]
	if parentIssue != nil {
		switch parentIssue.IssueType {
		case types.TypeFeature, types.TypeFeatureRequest:
			return filepath.Join(parentDir, "epics", issue.ID, issueFileName)
		case types.TypeEpic:
			return filepath.Join(parentDir, "tasks", issue.ID, issueFileName)
		}
	}
	return filepath.Join(parentDir, "children", issue.ID, issueFileName)
}

func parentID(issue *types.Issue) string {
	for _, dep := range issue.Dependencies {
		if dep != nil && dep.Type == types.DepParentChild {
			return dep.DependsOnID
		}
	}
	return ""
}

func (s *FileStorage) generateID(issueType types.IssueType) (string, error) {
	prefix := prefixForType(issueType)
	for i := 0; i < 100; i++ {
		suffix, err := randomSuffix(6)
		if err != nil {
			return "", err
		}
		id := prefix + "-" + suffix
		if !s.idExists(id) {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique ID after retries")
}

func (s *FileStorage) prepareIssuesForCreate(issues []*types.Issue) error {
	batchIDs := make(map[string]struct{}, len(issues))
	for i, issue := range issues {
		if issue == nil {
			return fmt.Errorf("issue %d is nil", i)
		}
		if issue.IssueType == "" {
			issue.IssueType = types.TypeTask
		}
		if issue.Status == "" {
			issue.Status = types.StatusOpen
		}
		if issue.ID == "" {
			for {
				id, err := s.generateID(issue.IssueType)
				if err != nil {
					return err
				}
				if _, exists := batchIDs[id]; !exists {
					issue.ID = id
					break
				}
			}
		}
		if err := ValidateFileIssueID(issue.ID); err != nil {
			return err
		}
		if s.idExists(issue.ID) {
			return fmt.Errorf("issue %s already exists", issue.ID)
		}
		if _, exists := batchIDs[issue.ID]; exists {
			return fmt.Errorf("duplicate ID within batch: %s", issue.ID)
		}
		batchIDs[issue.ID] = struct{}{}
		if err := issue.Validate(); err != nil {
			return fmt.Errorf("validation failed for issue %d: %w", i, err)
		}
	}
	return nil
}

func (s *FileStorage) prepareIssuesForImport(issues []*types.Issue, actor string) error {
	now := time.Now()
	batchIDs := make(map[string]struct{}, len(issues))
	batchExternalRefs := make(map[string]string)
	for i, issue := range issues {
		if issue == nil {
			return fmt.Errorf("issue %d is nil", i)
		}
		if issue.ID == "" {
			id, err := s.generateID(issue.IssueType)
			if err != nil {
				return err
			}
			issue.ID = id
		}
		if err := ValidateFileIssueID(issue.ID); err != nil {
			return err
		}
		if s.idExists(issue.ID) {
			return fmt.Errorf("issue %s already exists", issue.ID)
		}
		if _, exists := batchIDs[issue.ID]; exists {
			return fmt.Errorf("duplicate ID within batch: %s", issue.ID)
		}
		batchIDs[issue.ID] = struct{}{}
		if issue.ExternalRef != nil && *issue.ExternalRef != "" {
			ref := *issue.ExternalRef
			if existingID, exists := batchExternalRefs[ref]; exists && existingID != issue.ID {
				return fmt.Errorf("external_ref %s already belongs to %s; cannot assign to %s", ref, existingID, issue.ID)
			}
			batchExternalRefs[ref] = issue.ID
		}
		issue.SetDefaults()
		if issue.CreatedAt.IsZero() {
			issue.CreatedAt = now
		}
		if issue.UpdatedAt.IsZero() {
			issue.UpdatedAt = issue.CreatedAt
		}
		if issue.CreatedBy == "" {
			issue.CreatedBy = actor
		}
		if err := issue.Validate(); err != nil {
			return fmt.Errorf("validation failed for issue %d %s: %w", i, issue.ID, err)
		}
	}
	return nil
}

func prefixForType(issueType types.IssueType) string {
	switch issueType {
	case types.TypeFeature, types.TypeFeatureRequest:
		return "fr"
	case types.TypeEpic:
		return "ep"
	case types.TypeBug:
		return "bug"
	case types.TypeChore:
		return "ch"
	default:
		return "tk"
	}
}

func randomSuffix(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for i, b := range random {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

func (s *FileStorage) idExists(id string) bool {
	if _, ok := s.pathByID[id]; ok {
		return true
	}
	if issue, _ := s.GetIssue(context.Background(), id); issue != nil {
		return true
	}
	return false
}

func (s *FileStorage) absolutePathForID(id string) string {
	rel := s.pathByID[id]
	if rel == "" {
		return ""
	}
	return filepath.Join(s.taskLedgerDir, rel)
}

func writeFileIssueAtomic(path string, issue FileIssue) error {
	data, err := MarshalFileIssue(issue)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	_ = fsyncDir(filepath.Dir(path))
	return nil
}

func (s *FileStorage) withWriteLock(fn func() error) error {
	if s.lockDepth > 0 {
		return fn()
	}
	if err := os.MkdirAll(s.taskLedgerDir, projectDirMode); err != nil {
		return err
	}
	lockPath := filepath.Join(s.taskLedgerDir, ".lock")
	deadline := time.Now().Add(s.lockTimeout)
	for {
		err := writeLockFile(lockPath)
		if err == nil {
			if err := s.reload(); err != nil {
				_ = os.Remove(lockPath)
				return err
			}
			s.lockDepth++
			defer func() {
				s.lockDepth--
				_ = os.Remove(lockPath)
			}()
			return fn()
		}
		if !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("acquire lock: %w", err)
		}
		now := time.Now()
		stale, staleErr := lockFileIsStale(lockPath, s.staleLockAge(), now)
		if staleErr != nil {
			return fmt.Errorf("check lock %s: %w", lockPath, staleErr)
		}
		if stale {
			if err := os.Remove(lockPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("remove stale lock %s: %w", lockPath, err)
			}
			continue
		}
		if s.lockTimeout <= 0 || now.After(deadline) {
			return fmt.Errorf("timed out waiting for file storage lock %s", lockPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (s *FileStorage) staleLockAge() time.Duration {
	age := 2 * s.lockTimeout
	if age < 30*time.Second {
		return 30 * time.Second
	}
	return age
}

func lockFileIsStale(path string, maxAge time.Duration, now time.Time) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return now.Sub(info.ModTime()) > maxAge, nil
}

func writeLockFile(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, lockFileMode) // #nosec G304 -- lock path is constructed from the configured .task-ledger directory.
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "pid=%d\n", os.Getpid()); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func fsyncDir(path string) error {
	f, err := os.Open(path) // #nosec G304 -- path is a directory owned by the file backend.
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

type fileSnapshot map[string][]byte

func (s *FileStorage) snapshotIssueFiles() (fileSnapshot, error) {
	files, err := FindIssueFiles(s.taskLedgerDir)
	if err != nil {
		return nil, err
	}
	snapshot := make(fileSnapshot, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path) // #nosec G304 -- path comes from FindIssueFiles under the configured .task-ledger directory.
		if err != nil {
			return nil, err
		}
		snapshot[path] = data
	}
	return snapshot, nil
}

func (s *FileStorage) restoreSnapshot(snapshot fileSnapshot) error {
	current, err := FindIssueFiles(s.taskLedgerDir)
	if err != nil {
		return err
	}
	for _, path := range current {
		if _, ok := snapshot[path]; !ok {
			_ = os.Remove(path)
		}
	}
	for path, data := range snapshot {
		if err := os.MkdirAll(filepath.Dir(path), projectDirMode); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, projectFileMode); err != nil { // #nosec G306 -- issue files are project data and intentionally use the normal issue file mode.
			return err
		}
	}
	return nil
}

func (s *FileStorage) rollbackSnapshot(snapshot fileSnapshot, cause error) error {
	if restoreErr := s.restoreSnapshot(snapshot); restoreErr != nil {
		return fmt.Errorf("%w; rollback failed: %v", cause, restoreErr)
	}
	_ = s.reload()
	return cause
}

func (s *FileStorage) reload() error {
	s.issueIndex = newIssueIndex()
	s.pathByID = make(map[string]string)
	s.issuesByID = make(map[string]*types.Issue)
	return s.load()
}

func pruneEmptyDirs(start, stop string) {
	stop = filepath.Clean(stop)
	for dir := filepath.Clean(start); strings.HasPrefix(dir, stop) && dir != stop; dir = filepath.Dir(dir) {
		err := os.Remove(dir)
		if err != nil {
			return
		}
	}
}
