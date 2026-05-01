package main

import (
	"strings"

	"task-ledger/internal/types"
)

var fileBackendCommandAllowlist = map[string]bool{
	"blocked":      true,
	"close":        true,
	"comment":      true,
	"comments":     true,
	"count":        true,
	"create":       true,
	"delete":       true,
	"dep":          true,
	"doctor":       true,
	"epic":         true,
	"help":         true,
	"import-jsonl": true,
	"init":         true,
	"label":        true,
	"list":         true,
	"ready":        true,
	"reopen":       true,
	"search":       true,
	"show":         true,
	"update":       true,
	"version":      true,
}

func init() {
	pruneUnsupportedFileBackendCommands()
}

func pruneUnsupportedFileBackendCommands() {
	for _, cmd := range rootCmd.Commands() {
		if !fileBackendCommandAllowlist[cmd.Name()] {
			rootCmd.RemoveCommand(cmd)
		}
	}
}

func isFileBackendIssueType(issueType types.IssueType) bool {
	switch issueType {
	case types.TypeFeatureRequest, types.TypeFeature, types.TypeEpic, types.TypeTask, types.TypeBug, types.TypeChore:
		return true
	default:
		return false
	}
}

func fileBackendIssueTypesHelp() string {
	return strings.Join([]string{
		string(types.TypeFeatureRequest),
		string(types.TypeFeature),
		string(types.TypeEpic),
		string(types.TypeTask),
		string(types.TypeBug),
		string(types.TypeChore),
	}, ", ")
}
