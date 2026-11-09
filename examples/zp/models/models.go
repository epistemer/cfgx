// Package models stands in for zp's sqlc-generated models.
package models

// ZpTaskExecutionMode is shaped like the type sqlc generates for zp.
type ZpTaskExecutionMode string

const (
	ZpTaskExecutionModeSequential ZpTaskExecutionMode = "sequential"
	ZpTaskExecutionModeParallel   ZpTaskExecutionMode = "parallel"
)
