package engine

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/filter"
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

type DiagnosticProvider interface {
	Diagnose(ctx context.Context, errorLog string) (*DiagnosticReport, error)
	AssessRisk(ctx context.Context, symbol, path string) (*RiskAssessment, error)
}

type DiagnosticEngine struct {
	store    store.DiagnosticStore
	analyzer *AnalysisEngine
	impact   *ImpactEngine
	lspMgr   *lsp.Manager
	search   *SearchEngine
	logger   *slog.Logger
}

func NewDiagnosticEngine(
	s store.DiagnosticStore,
	a *AnalysisEngine,
	i *ImpactEngine,
	l *lsp.Manager,
	search *SearchEngine,
) *DiagnosticEngine {
	return &DiagnosticEngine{
		store:    s,
		analyzer: a,
		impact:   i,
		lspMgr:   l,
		search:   search,
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
	itPointers, _, _, _ := StreamSymbols(ctx, failingFile)
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

func (e *DiagnosticEngine) extractRCA(errorLog string) (string, int, [][]string, error) {
	re := regexp.MustCompile("(?m)" + filter.GoTestFailureRegex.String())
	allMatches := re.FindAllStringSubmatch(errorLog, -1)
	if len(allMatches) == 0 {
		return "", 0, nil, fmt.Errorf("could not identify failing file:line in log")
	}

	primaryMatch := allMatches[0]
	failingFileRaw := primaryMatch[1]
	lineNum, _ := strconv.Atoi(primaryMatch[2])

	failingFile, err := utils.ValidatePath(failingFileRaw)
	if err != nil {
		return "", 0, nil, err
	}
	return failingFile, lineNum, allMatches, nil
}

func (e *DiagnosticEngine) buildHealerContext(ctx context.Context, target *types.ASTPointer, failingFile, errorLog string, allMatches [][]string, originalCode string) string {
	var contextBuilder strings.Builder
	contextBuilder.WriteString(fmt.Sprintf("Failing File: %s\nTarget: %s\nError:\n%s\n\n", failingFile, target.Name, errorLog))

	for _, match := range allMatches {
		f := match[1]
		l, _ := strconv.Atoi(match[2])

		if e.lspMgr != nil {
			client, err := e.lspMgr.GetClient(ctx, f)
			if err == nil {
				hover, _ := client.Hover(ctx, lsp.HoverParams{
					TextDocumentPositionParams: lsp.TextDocumentPositionParams{
						TextDocument: lsp.TextDocumentIdentifier{URI: "file://" + f},
						Position:     lsp.Position{Line: l - 1, Character: 1},
					},
				})
				if hover != nil && hover.Contents.Value != "" {
					contextBuilder.WriteString(fmt.Sprintf("Context at %s:%d: %s\n", f, l, hover.Contents.Value))
				}
			}
		}
	}
	contextBuilder.WriteString("\nCode:\n" + originalCode)

	risk, _ := e.impact.Analyze(ctx, target.Name, failingFile, 1)
	if risk != nil {
		contextBuilder.WriteString(fmt.Sprintf("\n\nCurrent Risk Score: %.2f (%s)", risk.Target.RiskScore, risk.RiskLevel))
	}

	return contextBuilder.String()
}

// DiagnosticHUD representa la visión térmica/estructural del motor de diagnóstico.
type DiagnosticHUD struct {
	FailingSymbol      string
	LastCommit         string
	RiskLevel          string
	RiskScore          float64
	BlastRadius        int
	SimilarPatternPath string
}

// DiagnoseHUD ejecuta la visión de diagnóstico combinando Git, AST, Impacto y Bleve.
// Retorna la representación del HUD en formato ZON.
func (e *DiagnosticEngine) DiagnoseHUD(ctx context.Context, errorLog string) (*DiagnosticHUD, error) {
	if errorLog == "" {
		return nil, fmt.Errorf("empty error log")
	}

	// 1. Parseo: Extraer archivo y línea
	re := regexp.MustCompile("(?m)" + filter.GoTestFailureRegex.String())
	allMatches := re.FindAllStringSubmatch(errorLog, -1)
	if len(allMatches) == 0 {
		return nil, fmt.Errorf("could not identify failing file:line in log")
	}

	primaryMatch := allMatches[0]
	failingFileRaw := primaryMatch[1]
	lineNum, _ := strconv.Atoi(primaryMatch[2])

	failingFile, err := utils.ValidatePath(failingFileRaw)
	if err != nil {
		return nil, err
	}

	// 2. Visión de Tiempo (Provenance)
	var lastCommit string
	// Intentamos obtener la procedencia usando el root del proyecto.
	repoRoot := "." // Idealmente utils.GetRepoName(ctx) o similar, pero usamos "." como fallback
	provenance, err := GetFileProvenance(ctx, repoRoot, failingFile)
	if err == nil && len(provenance) >= lineNum && lineNum > 0 {
		// La línea es 1-indexed, el array también lo hacemos 1-indexed en la lógica o verificamos
		// GetFileProvenance devuelve slice, así que el índice sería lineNum - 1
		idx := lineNum - 1
		if idx < len(provenance) {
			lastCommit = provenance[idx].Commit + " (" + provenance[idx].EngineeringEra + ")"
		}
	} else {
		lastCommit = "Unknown"
	}

	// 3. Visión de Rayos X (AST)
	itPointers, _, _, err := StreamSymbols(ctx, failingFile)
	if err != nil {
		return nil, err
	}

	var symbol string
	for p := range itPointers {
		if lineNum >= p.StartLine && lineNum <= p.EndLine {
			symbol = p.Name
			break
		}
	}
	if symbol == "" {
		symbol = "Unknown"
	}

	// 4. Visión de Radar (Impacto)
	riskLevel := "Unknown"
	riskScore := 0.0
	blastRadius := 0
	if symbol != "Unknown" {
		impact, err := e.impact.Analyze(ctx, symbol, failingFile, 1)
		if err == nil && impact != nil {
			riskLevel = impact.RiskLevel
			riskScore = impact.Target.RiskScore
			blastRadius = len(impact.Callers)
		}
	}

	// 5. Visión Térmica (Similitud)
	similarPath := "None"
	if symbol != "Unknown" && e.search != nil {
		// Usamos HybridSearch para encontrar Logical Twins
		searchRes, err := e.search.HybridSearch(ctx, symbol, 10, 0)
		if err == nil && searchRes != nil {
			for _, sym := range searchRes.Symbols {
				if sym.Path != failingFile {
					similarPath = sym.Path
					break
				}
			}
		}
	}

	hud := DiagnosticHUD{
		FailingSymbol:      symbol,
		LastCommit:         lastCommit,
		RiskLevel:          riskLevel,
		RiskScore:          riskScore,
		BlastRadius:        blastRadius,
		SimilarPatternPath: similarPath,
	}

	return &hud, nil
}
