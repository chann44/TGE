package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const TypeScanRun = "scans:run"

type ScanRunPayload struct {
	ScanRunID int64  `json:"scan_run_id"`
	UserID    int64  `json:"user_id"`
	RepoID    int64  `json:"repo_id"`
	Trigger   string `json:"trigger"`
}

func NewScanRunTask(payload ScanRunPayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal scan run payload: %w", err)
	}
	return asynq.NewTask(TypeScanRun, body), nil
}

func ParseScanRunPayload(task *asynq.Task) (ScanRunPayload, error) {
	var payload ScanRunPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return ScanRunPayload{}, fmt.Errorf("unmarshal scan run payload: %w", err)
	}
	if payload.UserID == 0 || payload.RepoID == 0 {
		return ScanRunPayload{}, fmt.Errorf("invalid scan run payload")
	}
	if payload.Trigger == "" {
		payload.Trigger = "manual"
	}
	return payload, nil
}
