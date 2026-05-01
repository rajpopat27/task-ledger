package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"task-ledger/internal/types"
	"task-ledger/internal/validation"
)

func applyPriorityRangeFlags(cmd *cobra.Command, filter *types.IssueFilter) {
	if cmd.Flags().Changed("priority-min") {
		priorityMin := mustParsePriorityRangeFlag(cmd, "priority-min")
		filter.PriorityMin = &priorityMin
	}
	if cmd.Flags().Changed("priority-max") {
		priorityMax := mustParsePriorityRangeFlag(cmd, "priority-max")
		filter.PriorityMax = &priorityMax
	}
}

func mustParsePriorityRangeFlag(cmd *cobra.Command, name string) int {
	value, _ := cmd.Flags().GetString(name)
	priority, err := validation.ValidatePriority(value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing --%s: %v\n", name, err)
		os.Exit(1)
	}
	return priority
}

func applyLifecycleDateRangeFlags(cmd *cobra.Command, filter *types.IssueFilter) {
	applyTimeFlag(cmd, "created-after", func(t time.Time) { filter.CreatedAfter = &t })
	applyTimeFlag(cmd, "created-before", func(t time.Time) { filter.CreatedBefore = &t })
	applyTimeFlag(cmd, "updated-after", func(t time.Time) { filter.UpdatedAfter = &t })
	applyTimeFlag(cmd, "updated-before", func(t time.Time) { filter.UpdatedBefore = &t })
	applyTimeFlag(cmd, "closed-after", func(t time.Time) { filter.ClosedAfter = &t })
	applyTimeFlag(cmd, "closed-before", func(t time.Time) { filter.ClosedBefore = &t })
}

func applySchedulingDateRangeFlags(cmd *cobra.Command, filter *types.IssueFilter) {
	applyTimeFlag(cmd, "defer-after", func(t time.Time) { filter.DeferAfter = &t })
	applyTimeFlag(cmd, "defer-before", func(t time.Time) { filter.DeferBefore = &t })
	applyTimeFlag(cmd, "due-after", func(t time.Time) { filter.DueAfter = &t })
	applyTimeFlag(cmd, "due-before", func(t time.Time) { filter.DueBefore = &t })
}

func applyTimeFlag(cmd *cobra.Command, name string, assign func(time.Time)) {
	value, _ := cmd.Flags().GetString(name)
	if value == "" {
		return
	}
	parsed, err := parseTimeFlag(value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing --%s: %v\n", name, err)
		os.Exit(1)
	}
	assign(parsed)
}
