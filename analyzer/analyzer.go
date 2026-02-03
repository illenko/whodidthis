package analyzer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/illenko/whodidthis/models"
	"github.com/illenko/whodidthis/storage"
)

const maxAgenticIterations = 20

// Analyzer orchestrates AI-powered snapshot analysis using Gemini with agentic tool-calling
type Analyzer struct {
	gemini       *GeminiClient
	toolExecutor *ToolExecutor
	analysisRepo *storage.AnalysisRepository
	snapshots    *storage.SnapshotsRepository
	services     *storage.ServicesRepository

	mu                 sync.RWMutex
	running            bool
	currentSnapshotID  int64
	previousSnapshotID int64
	progress           string
	logger             *slog.Logger
}

// Config holds dependencies for creating an Analyzer
type Config struct {
	GeminiClient *GeminiClient
	ToolExecutor *ToolExecutor
	AnalysisRepo *storage.AnalysisRepository
	Snapshots    *storage.SnapshotsRepository
	Services     *storage.ServicesRepository
}

// New creates a new Analyzer service
func New(cfg Config) *Analyzer {
	return &Analyzer{
		gemini:       cfg.GeminiClient,
		toolExecutor: cfg.ToolExecutor,
		analysisRepo: cfg.AnalysisRepo,
		snapshots:    cfg.Snapshots,
		services:     cfg.Services,
		logger:       slog.Default().With("component", "analyzer"),
	}
}

// StartAnalysis begins analyzing the difference between two snapshots
func (a *Analyzer) StartAnalysis(ctx context.Context, currentID, previousID int64) (*models.SnapshotAnalysis, error) {
	// Validate both snapshots exist
	currentSnapshot, err := a.snapshots.GetByID(ctx, currentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current snapshot: %w", err)
	}
	if currentSnapshot == nil {
		return nil, fmt.Errorf("current snapshot %d not found", currentID)
	}

	previousSnapshot, err := a.snapshots.GetByID(ctx, previousID)
	if err != nil {
		return nil, fmt.Errorf("failed to get previous snapshot: %w", err)
	}
	if previousSnapshot == nil {
		return nil, fmt.Errorf("previous snapshot %d not found", previousID)
	}

	// Check if analysis already exists for this pair
	existing, err := a.analysisRepo.GetByPair(ctx, currentID, previousID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing analysis: %w", err)
	}
	if existing != nil && existing.Status == models.AnalysisStatusCompleted {
		a.logger.Info("returning existing completed analysis", "analysis_id", existing.ID)
		return existing, nil
	}

	// Check if another analysis is running
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil, fmt.Errorf("another analysis is already running (snapshots %d vs %d)", a.currentSnapshotID, a.previousSnapshotID)
	}
	a.running = true
	a.currentSnapshotID = currentID
	a.previousSnapshotID = previousID
	a.progress = "Initializing"
	a.mu.Unlock()

	// Create pending analysis record
	analysis, err := a.analysisRepo.Create(ctx, currentID, previousID)
	if err != nil {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		return nil, fmt.Errorf("failed to create analysis record: %w", err)
	}

	// Launch analysis in background goroutine
	go a.runAnalysis(analysis, currentSnapshot, previousSnapshot)

	// Return the analysis with status "running"
	analysis.Status = models.AnalysisStatusRunning
	return analysis, nil
}

// runAnalysis performs the actual AI-powered analysis in a background goroutine
func (a *Analyzer) runAnalysis(analysis *models.SnapshotAnalysis, current, previous *models.Snapshot) {
	// Use background context, not the request context which gets cancelled
	ctx := context.Background()

	// Ensure we clear the running state when done
	defer func() {
		a.mu.Lock()
		a.running = false
		a.currentSnapshotID = 0
		a.previousSnapshotID = 0
		a.progress = ""
		a.mu.Unlock()
	}()

	a.logger.Info("starting analysis",
		"analysis_id", analysis.ID,
		"current_snapshot", current.ID,
		"previous_snapshot", previous.ID,
	)

	// Update status to running
	analysis.Status = models.AnalysisStatusRunning
	if err := a.analysisRepo.Update(ctx, analysis); err != nil {
		a.logger.Error("failed to update analysis status to running", "error", err)
		// Continue anyway
	}

	// Build initial prompt with snapshot summaries
	prompt, err := a.buildPrompt(ctx, current, previous)
	if err != nil {
		a.logger.Error("failed to build prompt", "error", err)
		a.completeAnalysisWithError(ctx, analysis, err)
		return
	}

	a.updateProgress("Calling Gemini API")

	// Start conversation with Gemini
	resp, err := a.gemini.GenerateWithTools(ctx, prompt, toolDefinitions)
	if err != nil {
		a.logger.Error("failed to generate with tools", "error", err)
		a.completeAnalysisWithError(ctx, analysis, err)
		return
	}

	// Initialize conversation history
	history := []Content{
		{
			Role: "user",
			Parts: []Part{
				{Text: prompt},
			},
		},
	}

	// Agentic loop: continue while Gemini wants to call tools
	iteration := 0
	for resp.HasFunctionCall() {
		iteration++
		if iteration > maxAgenticIterations {
			err := fmt.Errorf("exceeded maximum iterations (%d)", maxAgenticIterations)
			a.logger.Error("agentic loop limit reached", "error", err)
			a.completeAnalysisWithError(ctx, analysis, err)
			return
		}

		// Get the function call
		functionCall := resp.GetFunctionCall()
		if functionCall == nil {
			a.logger.Error("function call is nil despite HasFunctionCall being true")
			break
		}

		a.logger.Info("executing tool",
			"iteration", iteration,
			"tool", functionCall.Name,
			"args", functionCall.Args,
		)

		a.updateProgress(fmt.Sprintf("Executing tool: %s (iteration %d)", functionCall.Name, iteration))

		// Execute the tool
		result, err := a.toolExecutor.Execute(ctx, functionCall)
		if err != nil {
			a.logger.Error("tool execution failed",
				"tool", functionCall.Name,
				"error", err,
			)
			// Record the error in the result
			result = map[string]any{
				"error": err.Error(),
			}
		}

		// Record the tool call in analysis
		analysis.ToolCalls = append(analysis.ToolCalls, models.ToolCall{
			Name:   functionCall.Name,
			Args:   functionCall.Args,
			Result: result,
		})

		// Update analysis with tool calls so far
		if err := a.analysisRepo.Update(ctx, analysis); err != nil {
			a.logger.Error("failed to update analysis with tool call", "error", err)
			// Continue anyway
		}

		// Add the model's response (with function call) to history
		history = append(history, resp.GetContent())

		// Continue conversation with tool result
		resp, err = a.gemini.ContinueWithToolResult(ctx, history, toolDefinitions, functionCall.Name, result)
		if err != nil {
			a.logger.Error("failed to continue with tool result", "error", err)
			a.completeAnalysisWithError(ctx, analysis, err)
			return
		}
	}

	a.updateProgress("Generating final analysis")

	// Get final text response
	finalText := resp.GetTextResponse()
	if finalText == "" {
		a.logger.Warn("received empty final response from Gemini")
		finalText = "No analysis generated."
	}

	a.logger.Info("analysis completed",
		"analysis_id", analysis.ID,
		"iterations", iteration,
		"tool_calls", len(analysis.ToolCalls),
	)

	// Update analysis with completion
	now := time.Now()
	analysis.Status = models.AnalysisStatusCompleted
	analysis.Result = finalText
	analysis.CompletedAt = &now

	if err := a.analysisRepo.Update(ctx, analysis); err != nil {
		a.logger.Error("failed to update analysis with final result", "error", err)
		return
	}

	a.updateProgress("Completed")
}

// buildPrompt constructs the system prompt with snapshot data
func (a *Analyzer) buildPrompt(ctx context.Context, current, previous *models.Snapshot) (string, error) {
	// Fetch services for current snapshot
	currentServices, err := a.services.List(ctx, current.ID, storage.ServiceListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list current services: %w", err)
	}

	// Fetch services for previous snapshot
	previousServices, err := a.services.List(ctx, previous.ID, storage.ServiceListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list previous services: %w", err)
	}

	// Build the prompt from the plan template
	prompt := fmt.Sprintf(`You are analyzing Prometheus metrics snapshots for a monitoring system.

You have EXACTLY 3 tools available. Do NOT call any other tools:
1. get_service_metrics(snapshot_id, service_name) - Get all metrics for a service in a snapshot
2. get_metric_labels(snapshot_id, service_name, metric_name) - Get labels for a specific metric
3. compare_services(current_snapshot_id, previous_snapshot_id, service_name) - Compare a service between two snapshots

These tools only accept the parameters listed above. Do not add extra parameters.

Current snapshot (ID: %d):
- Collected at: %s
- Total services: %d
- Total series: %d

Services in this snapshot:
%s

Previous snapshot (ID: %d):
- Collected at: %s
- Total services: %d
- Total series: %d

Services in previous snapshot:
%s

Your task:
1. Identify significant changes between snapshots
2. Look for anomalies (sudden spikes in series, new/removed services)
3. Use the 3 tools listed above to drill down into interesting services
4. Provide a concise analysis with actionable insights

When using tools, use the snapshot IDs provided above.
Keep your final analysis brief (3-5 bullet points) and focused on what operators need to know.`,
		current.ID,
		current.CollectedAt.Format(time.RFC3339),
		current.TotalServices,
		current.TotalSeries,
		formatServiceList(currentServices),
		previous.ID,
		previous.CollectedAt.Format(time.RFC3339),
		previous.TotalServices,
		previous.TotalSeries,
		formatServiceList(previousServices),
	)

	return prompt, nil
}

// formatServiceList formats a slice of services for the prompt
func formatServiceList(services []models.ServiceSnapshot) string {
	if len(services) == 0 {
		return "  (no services)"
	}

	result := ""
	for _, svc := range services {
		result += fmt.Sprintf("  - %s: %d series (%d metrics)\n", svc.ServiceName, svc.TotalSeries, svc.MetricCount)
	}
	return result
}

// completeAnalysisWithError marks the analysis as failed
func (a *Analyzer) completeAnalysisWithError(ctx context.Context, analysis *models.SnapshotAnalysis, err error) {
	now := time.Now()
	analysis.Status = models.AnalysisStatusFailed
	analysis.Error = err.Error()
	analysis.CompletedAt = &now

	if updateErr := a.analysisRepo.Update(ctx, analysis); updateErr != nil {
		a.logger.Error("failed to update analysis with error", "error", updateErr)
	}
}

// updateProgress updates the current progress string (thread-safe)
func (a *Analyzer) updateProgress(progress string) {
	a.mu.Lock()
	a.progress = progress
	a.mu.Unlock()
}

// GetAnalysis retrieves an existing analysis for a snapshot pair
func (a *Analyzer) GetAnalysis(ctx context.Context, currentID, previousID int64) (*models.SnapshotAnalysis, error) {
	return a.analysisRepo.GetByPair(ctx, currentID, previousID)
}

// ListAnalyses gets all analyses involving a specific snapshot
func (a *Analyzer) ListAnalyses(ctx context.Context, snapshotID int64) ([]models.SnapshotAnalysis, error) {
	return a.analysisRepo.ListBySnapshot(ctx, snapshotID)
}

// DeleteAnalysis removes an analysis (useful for regeneration)
func (a *Analyzer) DeleteAnalysis(ctx context.Context, currentID, previousID int64) error {
	return a.analysisRepo.Delete(ctx, currentID, previousID)
}

// GetGlobalStatus returns the current analysis status
func (a *Analyzer) GetGlobalStatus() models.AnalysisGlobalStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return models.AnalysisGlobalStatus{
		Running:            a.running,
		CurrentSnapshotID:  a.currentSnapshotID,
		PreviousSnapshotID: a.previousSnapshotID,
		Progress:           a.progress,
	}
}
