package tools

import (
	"encoding/json"
	"fmt"
	"sync"
)

type ShutdownReq struct {
	Target string `json:"target"`
	Status string `json:"status"` // pending, approved, rejected
}
type PlanReq struct {
	From   string `json:"from"`
	Plan   string `json:"plan"`
	Status string `json:"status"` // pending, approved, rejected
}

var (
	trackerMu        sync.Mutex
	shutdownRequests = make(map[string]*ShutdownReq)
	planRequests     = make(map[string]*PlanReq)
)

func HandleShutdownRequest(teammate string, bus *MessageBus) string {
	reqID := shortID() // 复用之前 BackgroundManager 里的短 UUID 生成函数

	trackerMu.Lock()
	shutdownRequests[reqID] = &ShutdownReq{Target: teammate, Status: "pending"}
	trackerMu.Unlock()

	extra := map[string]interface{}{"request_id": reqID}
	bus.Send("lead", teammate, "Please shut down gracefully.", "shutdown_request", extra)

	return fmt.Sprintf("Shutdown request %s sent to '%s' (status: pending)", reqID, teammate)
}

// Lead 操作：审批计划
func HandlePlanReview(reqID string, approve bool, feedback string, bus *MessageBus) string {
	trackerMu.Lock()
	req, exists := planRequests[reqID]
	if !exists {
		trackerMu.Unlock()
		return fmt.Sprintf("Error: Unknown plan request_id '%s'", reqID)
	}
	if approve {
		req.Status = "approved"
	} else {
		req.Status = "rejected"
	}
	trackerMu.Unlock()

	extra := map[string]interface{}{
		"request_id": reqID,
		"approve":    approve,
		"feedback":   feedback,
	}
	bus.Send("lead", req.From, feedback, "plan_approval_response", extra)

	return fmt.Sprintf("Plan %s for '%s'", req.Status, req.From)
}
func CheckShutdownStatus(reqID string) string {
	trackerMu.Lock()
	defer trackerMu.Unlock()

	if req, exists := shutdownRequests[reqID]; exists {
		b, _ := json.Marshal(req)
		return string(b)
	}
	return `{"error": "not found"}`
}
