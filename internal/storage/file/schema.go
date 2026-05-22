// Package file implements a direct JSON-file storage backend.
package file

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"task-ledger/internal/types"
)

const SchemaVersion = 1

// FileIssue is the stable on-disk representation stored in issue.json.
type FileIssue struct {
	SchemaVersion      int                 `json:"schema_version"`
	ID                 string              `json:"id"`
	Type               string              `json:"type"`
	Title              string              `json:"title"`
	Status             string              `json:"status"`
	Priority           int                 `json:"priority"`
	Parent             string              `json:"parent,omitempty"`
	FeatureRequest     string              `json:"feature_request,omitempty"`
	Epic               string              `json:"epic,omitempty"`
	Deps               []string            `json:"deps"`
	Dependencies       []*types.Dependency `json:"dependencies,omitempty"`
	Labels             []string            `json:"labels"`
	Assignee           string              `json:"assignee,omitempty"`
	Owner              string              `json:"owner,omitempty"`
	EstimatedMinutes   *int                `json:"estimated_minutes,omitempty"`
	ExternalRef        string              `json:"external_ref,omitempty"`
	SourceSystem       string              `json:"source_system,omitempty"`
	Description        string              `json:"description,omitempty"`
	Design             string              `json:"design,omitempty"`
	AcceptanceCriteria string              `json:"acceptance_criteria,omitempty"`
	Notes              string              `json:"notes,omitempty"`
	Comments           []*types.Comment    `json:"comments"`
	AgentWF            json.RawMessage     `json:"agentwf,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	CreatedBy          string              `json:"created_by,omitempty"`
	UpdatedAt          time.Time           `json:"updated_at"`
	ClosedAt           *time.Time          `json:"closed_at"`
	CloseReason        string              `json:"close_reason,omitempty"`
	ClosedBySession    string              `json:"closed_by_session,omitempty"`
	DueAt              *time.Time          `json:"due_at,omitempty"`
	DeferUntil         *time.Time          `json:"defer_until,omitempty"`
	DeletedAt          *time.Time          `json:"deleted_at,omitempty"`
	DeletedBy          string              `json:"deleted_by,omitempty"`
	DeleteReason       string              `json:"delete_reason,omitempty"`
	OriginalType       string              `json:"original_type,omitempty"`
}

// FromTypesIssue converts the runtime issue representation to the on-disk form.
func FromTypesIssue(issue *types.Issue) FileIssue {
	if issue == nil {
		return FileIssue{SchemaVersion: SchemaVersion}
	}

	parent := ""
	deps := make([]string, 0)
	dependencies := make([]*types.Dependency, 0, len(issue.Dependencies))
	for _, dep := range issue.Dependencies {
		if dep == nil {
			continue
		}
		depCopy := *dep
		if depCopy.IssueID == "" {
			depCopy.IssueID = issue.ID
		}
		dependencies = append(dependencies, &depCopy)
		switch dep.Type {
		case types.DepParentChild:
			parent = dep.DependsOnID
		case types.DepBlocks:
			deps = append(deps, dep.DependsOnID)
		}
	}
	sort.Strings(deps)
	labels := append([]string(nil), issue.Labels...)
	if labels == nil {
		labels = []string{}
	}
	sort.Strings(labels)

	externalRef := ""
	if issue.ExternalRef != nil {
		externalRef = *issue.ExternalRef
	}

	return FileIssue{
		SchemaVersion:      SchemaVersion,
		ID:                 issue.ID,
		Type:               string(issue.IssueType),
		Title:              issue.Title,
		Status:             string(issue.Status),
		Priority:           issue.Priority,
		Parent:             parent,
		FeatureRequest:     "",
		Epic:               "",
		Deps:               deps,
		Dependencies:       dependencies,
		Labels:             labels,
		Assignee:           issue.Assignee,
		Owner:              issue.Owner,
		EstimatedMinutes:   issue.EstimatedMinutes,
		ExternalRef:        externalRef,
		SourceSystem:       issue.SourceSystem,
		Description:        issue.Description,
		Design:             issue.Design,
		AcceptanceCriteria: issue.AcceptanceCriteria,
		Notes:              issue.Notes,
		Comments:           cloneComments(issue.Comments),
		AgentWF:            cloneRawMessage(issue.AgentWF),
		CreatedAt:          issue.CreatedAt,
		CreatedBy:          issue.CreatedBy,
		UpdatedAt:          issue.UpdatedAt,
		ClosedAt:           issue.ClosedAt,
		CloseReason:        issue.CloseReason,
		ClosedBySession:    issue.ClosedBySession,
		DueAt:              issue.DueAt,
		DeferUntil:         issue.DeferUntil,
		DeletedAt:          issue.DeletedAt,
		DeletedBy:          issue.DeletedBy,
		DeleteReason:       issue.DeleteReason,
		OriginalType:       issue.OriginalType,
	}
}

// ToTypesIssue converts the on-disk form to the runtime issue representation.
func (fi FileIssue) ToTypesIssue() (*types.Issue, error) {
	if fi.SchemaVersion == 0 {
		fi.SchemaVersion = SchemaVersion
	}
	if fi.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported schema_version %d", fi.SchemaVersion)
	}
	if fi.ID == "" {
		return nil, fmt.Errorf("id is required")
	}
	if fi.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	issueType := types.IssueType(fi.Type)
	if issueType == "" {
		issueType = types.TypeTask
	}
	status := types.Status(fi.Status)
	if status == "" {
		status = types.StatusOpen
	}

	var externalRef *string
	if fi.ExternalRef != "" {
		ref := fi.ExternalRef
		externalRef = &ref
	}

	dependencies := make([]*types.Dependency, 0, len(fi.Dependencies)+len(fi.Deps)+1)
	seen := make(map[string]bool)
	for _, dep := range fi.Dependencies {
		if dep == nil {
			continue
		}
		depCopy := *dep
		if depCopy.IssueID == "" {
			depCopy.IssueID = fi.ID
		}
		key := dependencyKey(&depCopy)
		dependencies = append(dependencies, &depCopy)
		seen[key] = true
	}
	for _, depID := range fi.Deps {
		dep := &types.Dependency{IssueID: fi.ID, DependsOnID: depID, Type: types.DepBlocks}
		key := dependencyKey(dep)
		if depID != "" && !seen[key] {
			dependencies = append(dependencies, dep)
			seen[key] = true
		}
	}
	if fi.Parent != "" {
		dep := &types.Dependency{IssueID: fi.ID, DependsOnID: fi.Parent, Type: types.DepParentChild}
		if key := dependencyKey(dep); !seen[key] {
			dependencies = append(dependencies, dep)
		}
	}

	issue := &types.Issue{
		ID:                 fi.ID,
		Title:              fi.Title,
		Description:        fi.Description,
		Design:             fi.Design,
		AcceptanceCriteria: fi.AcceptanceCriteria,
		Notes:              fi.Notes,
		Status:             status,
		Priority:           fi.Priority,
		IssueType:          issueType,
		Assignee:           fi.Assignee,
		Owner:              fi.Owner,
		EstimatedMinutes:   fi.EstimatedMinutes,
		ExternalRef:        externalRef,
		SourceSystem:       fi.SourceSystem,
		CreatedAt:          fi.CreatedAt,
		CreatedBy:          fi.CreatedBy,
		UpdatedAt:          fi.UpdatedAt,
		ClosedAt:           fi.ClosedAt,
		CloseReason:        fi.CloseReason,
		ClosedBySession:    fi.ClosedBySession,
		DueAt:              fi.DueAt,
		DeferUntil:         fi.DeferUntil,
		Labels:             append([]string(nil), fi.Labels...),
		Dependencies:       dependencies,
		Comments:           cloneComments(fi.Comments),
		AgentWF:            cloneRawMessage(fi.AgentWF),
		DeletedAt:          fi.DeletedAt,
		DeletedBy:          fi.DeletedBy,
		DeleteReason:       fi.DeleteReason,
		OriginalType:       fi.OriginalType,
	}
	issue.SetDefaults()
	return issue, nil
}

// MarshalFileIssue renders an issue.json with stable formatting.
func MarshalFileIssue(issue FileIssue) ([]byte, error) {
	if issue.SchemaVersion == 0 {
		issue.SchemaVersion = SchemaVersion
	}
	data, err := json.MarshalIndent(issue, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func dependencyKey(dep *types.Dependency) string {
	return string(dep.Type) + "\x00" + dep.IssueID + "\x00" + dep.DependsOnID
}

func cloneComments(comments []*types.Comment) []*types.Comment {
	if len(comments) == 0 {
		return []*types.Comment{}
	}
	out := make([]*types.Comment, 0, len(comments))
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		commentCopy := *comment
		out = append(out, &commentCopy)
	}
	return out
}

func cloneRawMessage(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}
