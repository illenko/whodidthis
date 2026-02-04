package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/illenko/whodidthis/config"
	"github.com/illenko/whodidthis/models"
	"github.com/illenko/whodidthis/storage"
	"google.golang.org/genai"
)

const maxAgenticIterations = 20
const defaultGeminiModel = "gemini-2.5-pro"

type Analyzer struct {
	client       *genai.Client
	model        string
	geminiConfig config.GeminiConfig
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

type Config struct {
	Gemini       config.GeminiConfig
	ToolExecutor *ToolExecutor
	AnalysisRepo *storage.AnalysisRepository
	Snapshots    *storage.SnapshotsRepository
	Services     *storage.ServicesRepository
}

func New(ctx context.Context, cfg Config) (*Analyzer, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.Gemini.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	model := cfg.Gemini.Model
	if model == "" {
		model = defaultGeminiModel
	}

	return &Analyzer{
		client:       client,
		model:        model,
		geminiConfig: cfg.Gemini,
		toolExecutor: cfg.ToolExecutor,
		analysisRepo: cfg.AnalysisRepo,
		snapshots:    cfg.Snapshots,
		services:     cfg.Services,
		logger:       slog.Default().With("component", "analyzer"),
	}, nil
}

func (a *Analyzer) StartAnalysis(ctx context.Context, currentID, previousID int64) (*models.SnapshotAnalysis, error) {
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

	existing, err := a.analysisRepo.GetByPair(ctx, currentID, previousID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing analysis: %w", err)
	}
	if existing != nil && existing.Status == models.AnalysisStatusCompleted {
		a.logger.Info("returning existing completed analysis", "analysis_id", existing.ID)
		return existing, nil
	}

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

	analysis, err := a.analysisRepo.Create(ctx, currentID, previousID)
	if err != nil {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		return nil, fmt.Errorf("failed to create analysis record: %w", err)
	}

	go a.runAnalysis(analysis, currentSnapshot, previousSnapshot)

	analysis.Status = models.AnalysisStatusRunning
	return analysis, nil
}

func (a *Analyzer) runAnalysis(analysis *models.SnapshotAnalysis, current, previous *models.Snapshot) {
	ctx := context.Background()

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

	analysis.Status = models.AnalysisStatusRunning
	if err := a.analysisRepo.Update(ctx, analysis); err != nil {
		a.logger.Error("failed to update analysis status to running", "error", err)
		// Continue anyway
	}

	prompt, err := a.buildPrompt(ctx, current, previous)
	if err != nil {
		a.logger.Error("failed to build prompt", "error", err)
		a.completeAnalysisWithError(ctx, analysis, err)
		return
	}

	a.updateProgress("Calling Gemini API")

	temp := a.geminiConfig.Chat.Temperature
	genaiConfig := &genai.GenerateContentConfig{
		Temperature:     &temp,
		MaxOutputTokens: a.geminiConfig.Chat.MaxOutputTokens,
		Tools:           []*genai.Tool{getGenaiToolDefinitions()},
	}
	chatSession, err := a.client.Chats.Create(ctx, a.model, genaiConfig, nil)
	if err != nil {
		a.logger.Error("failed to create chat session", "error", err)
		a.completeAnalysisWithError(ctx, analysis, err)
		return
	}

	// Send initial prompt
	resp, err := chatSession.SendMessage(ctx, genai.Part{Text: prompt})
	if err != nil {
		a.logger.Error("failed to send initial prompt to Gemini", "error", err)
		a.completeAnalysisWithError(ctx, analysis, err)
		return
	}

	for i := 0; i < maxAgenticIterations; i++ {
		if resp.Candidates == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			err = fmt.Errorf("received an empty response from Gemini")
			a.logger.Error("empty response", "error", err)
			a.completeAnalysisWithError(ctx, analysis, err)
			return
		}

		// Look for a function call
		var functionCall *genai.FunctionCall
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.FunctionCall != nil {
				functionCall = part.FunctionCall
				break
			}
		}

		if functionCall == nil {
			// No function call, analysis is complete
			break
		}

		a.logger.Info("executing tool", "iteration", i+1, "tool", functionCall.Name, "args", functionCall.Args)
		a.updateProgress(fmt.Sprintf("Executing tool: %s (iteration %d)", functionCall.Name, i+1))

		result, err := a.toolExecutor.Execute(ctx, functionCall.Name, functionCall.Args)
		if err != nil {
			a.logger.Error("tool execution failed", "tool", functionCall.Name, "error", err)
			result = map[string]any{"error": err.Error()}
		}

		analysis.ToolCalls = append(analysis.ToolCalls, models.ToolCall{
			Name:   functionCall.Name,
			Args:   functionCall.Args,
			Result: result,
		})

		if err := a.analysisRepo.Update(ctx, analysis); err != nil {
			a.logger.Error("failed to update analysis with tool call", "error", err)
		}

		// Send tool result back to Gemini
		responseMap, err := toMap(result)
		if err != nil {
			a.logger.Error("failed to convert tool result to map", "error", err)
			responseMap = map[string]any{"error": err.Error()}
		}
		resp, err = chatSession.SendMessage(ctx, genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name:     functionCall.Name,
				Response: responseMap,
			},
		})
		if err != nil {
			a.logger.Error("failed to send tool result to Gemini", "error", err)
			a.completeAnalysisWithError(ctx, analysis, err)
			return
		}
	}

	a.updateProgress("Generating final analysis")

	var finalText string
	if resp != nil && len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" && !part.Thought {
				finalText += part.Text
			}
		}
	}

	if finalText == "" {
		partsCount := 0
		thoughtCount := 0
		if resp != nil && len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			for _, part := range resp.Candidates[0].Content.Parts {
				partsCount++
				if part.Thought {
					thoughtCount++
				}
			}
		}
		a.logger.Warn("empty final response from Gemini",
			"parts_count", partsCount,
			"thought_parts", thoughtCount,
		)
		finalText = "No analysis generated."
	}

	a.logger.Info("analysis completed",
		"analysis_id", analysis.ID,
		"tool_calls", len(analysis.ToolCalls),
	)

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

func getGenaiToolDefinitions() *genai.Tool {
	return &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name:        "get_service_metrics",
				Description: "Get all metrics for a service in a snapshot",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"snapshot_id":  {Type: genai.TypeInteger, Description: "ID of the snapshot"},
						"service_name": {Type: genai.TypeString, Description: "Name of the service"},
					},
					Required: []string{"snapshot_id", "service_name"},
				},
			},
			{
				Name:        "get_metric_labels",
				Description: "Get all labels for a specific metric",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"snapshot_id":  {Type: genai.TypeInteger, Description: "ID of the snapshot"},
						"service_name": {Type: genai.TypeString, Description: "Name of the service"},
						"metric_name":  {Type: genai.TypeString, Description: "Name of the metric"},
					},
					Required: []string{"snapshot_id", "service_name", "metric_name"},
				},
			},
			{
				Name:        "compare_services",
				Description: "Compare a service between two snapshots to see added/removed metrics and series count changes",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"current_snapshot_id":  {Type: genai.TypeInteger, Description: "ID of the current snapshot"},
						"previous_snapshot_id": {Type: genai.TypeInteger, Description: "ID of the previous snapshot"},
						"service_name":         {Type: genai.TypeString, Description: "Name of the service"},
					},
					Required: []string{"current_snapshot_id", "previous_snapshot_id", "service_name"},
				},
			},
		},
	}
}

func (a *Analyzer) buildPrompt(ctx context.Context, current, previous *models.Snapshot) (string, error) {
	currentServices, err := a.services.List(ctx, current.ID, storage.ServiceListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list current services: %w", err)
	}

	previousServices, err := a.services.List(ctx, previous.ID, storage.ServiceListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list previous services: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert monitoring system analyzer specializing in Prometheus metrics analysis. Your goals:
1. Identify significant changes between two snapshots
2. Detect high cardinality issues and anti-patterns (IDs, UUIDs, URLs in labels)

# Available Tools

You have EXACTLY 3 tools. Do NOT attempt to call any other tools or add parameters not listed:

1. get_service_metrics(snapshot_id, service_name)
   - Returns: All metrics for the specified service in the given snapshot
   
2. get_metric_labels(snapshot_id, service_name, metric_name)
   - Returns: All label combinations for a specific metric
   
3. compare_services(current_snapshot_id, previous_snapshot_id, service_name)
   - Returns: Comparison showing added/removed metrics and series count changes
---
Current snapshot (ID: %d):
- Collected at: %s
- Total services: %d
- Total series: %d
Services in this snapshot:
%s
---
Previous snapshot (ID: %d):
- Collected at: %s
- Total services: %d
- Total series: %d
Services in previous snapshot:
%s
---
# Analysis Strategy

## Phase 1: Change Detection (2-3 tool calls)
- Use compare_services on 2-3 services with notable series count differences
- Identify new/removed services from the lists above (no tool needed)

## Phase 2: Cardinality Analysis (3-4 tool calls)
**CRITICAL**: Focus on detecting anti-patterns in the CURRENT snapshot:

For services with >1000 series OR >50 percents series growth:
1. Use get_service_metrics to identify metrics with high series counts
2. Use get_metric_labels on metrics with >100 series to examine label patterns

**Red flags to detect:**
- Label values containing UUIDs/GUIDs (patterns: 8-4-4-4-12 hex digits)
- Transaction/payment/request IDs in labels (numeric IDs >6 digits, alphanumeric codes)
- User IDs, account IDs, merchant IDs in labels
- URLs or paths with variable IDs (e.g., /api/transactions/12345/status)
- Timestamps or dates in label values
- Session tokens or correlation IDs
- Email addresses or personal identifiers

**Healthy patterns:**
- Bounded enums (status: success/failed/pending)
- Service names, environment, region, availability zone
- HTTP methods, response codes (2xx, 4xx, 5xx ranges)
- Provider names (limited set)
- Payment methods (card, wallet, bank_transfer - limited set)

## Phase 3: Stop Condition
- Never call the same tool with identical parameters twice
- Stop after 7-8 total tool calls or when you have enough data
- If a tool returns no useful insights, move to different service/metric

# Output Format

## 🚨 High Cardinality Issues (if found)
For each problematic metric:
- **Metric**: service_name.metric_name
- **Series count**: X
- **Problem**: [ID pattern in label_name: sample values]
- **Impact**: Estimated memory/storage overhead
- **Fix**: Remove label or use constant value

## 📊 Significant Changes
**Critical** (1-2 points):
- New/removed services, >50 percents series changes, new metric types

**Notable** (1-2 points):
- 20-50 percents series changes, cardinality increases

## ✅ Recommendations
Priority-ordered action items (max 3):
1. [Most urgent - usually cardinality fixes]
2. [Investigation needed]
3. [Monitoring adjustments]

Keep total analysis under 200 words. Prioritize cardinality issues over normal changes.

# Detection Heuristics

When examining label values with get_metric_labels:

**UUID/GUID patterns:**
- 32 hex chars with/without dashes: 550e8400-e29b-41d4-a716-446655440000
- Look for: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}

**ID patterns:**
- Long numeric sequences: transaction_id="123456789012"
- Alphanumeric codes: payment_id="PAY_abc123xyz456"
- Prefixed IDs: merchant_id="MER_12345"

**URL/Path patterns:**
- /api/users/12345/transactions
- /payments/550e8400-e29b-41d4-a716-446655440000/status

**Safe cardinality check:**
If a label has >50 unique values, it's likely unbounded and needs investigation.

# Important Constraints

- Use ONLY the snapshot IDs provided above
- Maximum %d tool calls total
- Prioritize CURRENT snapshot cardinality analysis over historical comparison
- Assume operator understands Prometheus and payment systems
- Be specific: show actual problematic label values as examples`,
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
		maxAgenticIterations,
	)

	return prompt, nil
}

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

func (a *Analyzer) completeAnalysisWithError(ctx context.Context, analysis *models.SnapshotAnalysis, err error) {
	now := time.Now()
	analysis.Status = models.AnalysisStatusFailed
	analysis.Error = err.Error()
	analysis.CompletedAt = &now

	if updateErr := a.analysisRepo.Update(ctx, analysis); updateErr != nil {
		a.logger.Error("failed to update analysis with error", "error", updateErr)
	}
}

func toMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (a *Analyzer) updateProgress(progress string) {
	a.mu.Lock()
	a.progress = progress
	a.mu.Unlock()
}

func (a *Analyzer) GetAnalysis(ctx context.Context, currentID, previousID int64) (*models.SnapshotAnalysis, error) {
	return a.analysisRepo.GetByPair(ctx, currentID, previousID)
}

func (a *Analyzer) ListAnalyses(ctx context.Context, snapshotID int64) ([]models.SnapshotAnalysis, error) {
	return a.analysisRepo.ListBySnapshot(ctx, snapshotID)
}

func (a *Analyzer) DeleteAnalysis(ctx context.Context, currentID, previousID int64) error {
	return a.analysisRepo.Delete(ctx, currentID, previousID)
}

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
