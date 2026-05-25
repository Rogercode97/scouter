package engine

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rogercode97/scouter/internal/filter"
	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/Rogercode97/scouter/internal/types"
	"github.com/Rogercode97/scouter/internal/utils"
)

// DiagnosticReport represents the "Technical Truth" kernel of a failure.
type DiagnosticReport struct {
	FailingSymbol string
	FailingFile   string
	Line          int
	ErrorLog      string
	Context       string
	Risk          *types.ImpactResult
	Insights      []string // Historical insights from Engram
}

// RiskAssessment consolidates centrality and blast radius metrics.
type RiskAssessment struct {
	Symbol     string
	Path       string
	RiskScore  float64
	Centrality int
	Callers    int
	RiskLevel  string
}

// DiagnosticProvider defines the deep interface for technical quality operations.
type DiagnosticProvider interface {
	Diagnose(ctx context.Context, errorLog string) (*DiagnosticReport, error)
	Heal(ctx context.Context, report *DiagnosticReport, messenger Messenger) (*types.HealResult, error)
	AssessRisk(ctx context.Context, symbol, path string) (*RiskAssessment, error)
}

// DiagnosticEngine implements DiagnosticProvider by orchestrating specialized engines.
type DiagnosticEngine struct {
	store    store.DiagnosticStore
	analyzer *AnalysisEngine
	impact   *ImpactEngine
	healer   *HealerEngine
	lspMgr   *lsp.Manager
	logger   *slog.Logger
}

func NewDiagnosticEngine(
	s store.DiagnosticStore,
	a *AnalysisEngine,
	i *ImpactEngine,
	h *HealerEngine,
	l *lsp.Manager,
) *DiagnosticEngine {
	return &DiagnosticEngine{
		store:    s,
		analyzer: a,
		impact:   i,
		healer:   h,
		lspMgr:   l,
		logger:   slog.Default(),
	}
}

// Diagnose parses logs and enriches the failure context.
func (e *DiagnosticEngine) Diagnose(ctx context.Context, errorLog string) (*DiagnosticReport, error) {
	re := regexp.MustCompile("(?m)" + filter.GoTestFailureRegex.String())
	allMatches := re.FindAllStringSubmatch(errorLog, -1)
	if len(allMatches) == 0 {
		return nil, fmt.Errorf("could not identify failing file:line in log")
	}

	primary := allMatches[0]
	failingFileRaw := primary[1]
	line, _ := strconv.Atoi(primary[2])

	failingFile, err := utils.ValidatePath(failingFileRaw)
	if err != nil {
		return nil, err
	}

	// Resolve Symbol
	itPointers, _, _ := StreamSymbols(ctx, failingFile)
	var symbol string
	for p := range itPointers {
		if line >= p.StartLine && line <= p.EndLine {
			symbol = p.Name
			break
		}
	}

	report := &DiagnosticReport{
		FailingSymbol: symbol,
		FailingFile:   failingFile,
		Line:          line,
		ErrorLog:      errorLog,
	}

	// Enrich with Impact
	if symbol != "" {
		report.Risk, _ = e.impact.Analyze(ctx, symbol, failingFile, 1)
	}

	// 🧠 ADR-0003: REQUEST HISTORICAL INSIGHTS FROM ENGRAM
	// In a real implementation, this would use Messenger to query Engram via the Agent.
	// For now, we seed it as a structural placeholder.
	report.Insights = []string{
		fmt.Sprintf("Historical check requested for symbol: %s", symbol),
	}

	return report, nil
}

// Heal delegates the actual fix to the HealerEngine using the enriched report.
func (e *DiagnosticEngine) Heal(ctx context.Context, report *DiagnosticReport, messenger Messenger) (*types.HealResult, error) {
	if e.healer == nil {
		return nil, fmt.Errorf("healer engine not initialized")
	}

	// Inject messenger bridge
	if messenger != nil {
		e.healer.DoFixRequest = func(ctx context.Context, prompt string) (string, error) {
			// Prepend insights to the prompt
			enrichedPrompt := "Historical Insights:\n" + strings.Join(report.Insights, "\n") + "\n\n" + prompt
			return messenger.Ask(ctx, "You are an autonomous Go fixing agent.", enrichedPrompt)
		}
	}

	return e.healer.Fix(ctx, report.ErrorLog)
}

// AssessRisk provides a unified risk assessment.
func (e *DiagnosticEngine) AssessRisk(ctx context.Context, symbol, path string) (*RiskAssessment, error) {
	impact, err := e.impact.Analyze(ctx, symbol, path, 3)
	if err != nil {
		return nil, err
	}

	return &RiskAssessment{
		Symbol:     symbol,
		Path:       path,
		RiskScore:  impact.Target.RiskScore,
		RiskLevel:  impact.RiskLevel,
		Callers:    len(impact.Callers),
		Centrality: int(impact.Target.Metrics.Centrality),
	}, nil
}
