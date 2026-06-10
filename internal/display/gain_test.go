package display

import (
	"context"
	"testing"

	"github.com/Rogercode97/scouter/internal/telemetry"
)

// MockTelemetryProvider implementa la interfaz TelemetryProvider para pruebas.
type MockTelemetryProvider struct {
	GetSummaryFunc   func(ctx context.Context) (*telemetry.Summary, error)
	GetDailyFunc     func(ctx context.Context, days int) ([]telemetry.DayStats, error)
	GetByCommandFunc func(ctx context.Context, limit int) ([]telemetry.CommandStats, error)
	GetWeeklyFunc    func(ctx context.Context, weeks int) ([]telemetry.PeriodStats, error)
	GetMonthlyFunc   func(ctx context.Context, months int) ([]telemetry.PeriodStats, error)
	GetRecentFunc    func(ctx context.Context, limit int) ([]telemetry.CommandRecord, error)
}

func (m *MockTelemetryProvider) GetSummary(ctx context.Context) (*telemetry.Summary, error) {
	if m.GetSummaryFunc != nil {
		return m.GetSummaryFunc(ctx)
	}
	return &telemetry.Summary{}, nil
}

func (m *MockTelemetryProvider) GetDaily(ctx context.Context, days int) ([]telemetry.DayStats, error) {
	if m.GetDailyFunc != nil {
		return m.GetDailyFunc(ctx, days)
	}
	return []telemetry.DayStats{}, nil
}

func (m *MockTelemetryProvider) GetByCommand(ctx context.Context, limit int) ([]telemetry.CommandStats, error) {
	if m.GetByCommandFunc != nil {
		return m.GetByCommandFunc(ctx, limit)
	}
	return []telemetry.CommandStats{}, nil
}

func (m *MockTelemetryProvider) GetWeekly(ctx context.Context, weeks int) ([]telemetry.PeriodStats, error) {
	if m.GetWeeklyFunc != nil {
		return m.GetWeeklyFunc(ctx, weeks)
	}
	return []telemetry.PeriodStats{}, nil
}

func (m *MockTelemetryProvider) GetMonthly(ctx context.Context, months int) ([]telemetry.PeriodStats, error) {
	if m.GetMonthlyFunc != nil {
		return m.GetMonthlyFunc(ctx, months)
	}
	return []telemetry.PeriodStats{}, nil
}

func (m *MockTelemetryProvider) GetRecent(ctx context.Context, limit int) ([]telemetry.CommandRecord, error) {
	if m.GetRecentFunc != nil {
		return m.GetRecentFunc(ctx, limit)
	}
	return []telemetry.CommandRecord{}, nil
}

func TestRunGain_DailyFlag(t *testing.T) {
	// 1. Arrange: Configuramos el mock con datos predecibles
	mock := &MockTelemetryProvider{
		GetSummaryFunc: func(ctx context.Context) (*telemetry.Summary, error) {
			return &telemetry.Summary{
				TotalSaved:    15000,
				AvgSavings:    55.5,
				TotalCommands: 10,
			}, nil
		},
		GetDailyFunc: func(ctx context.Context, days int) ([]telemetry.DayStats, error) {
			if days != 7 {
				t.Errorf("Se esperaba que solicitara 7 dias por defecto si no se pasa parametro o 7 por inicializacion, se solicitó: %d", days)
			}
			return []telemetry.DayStats{
				{
					Day:          "2023-10-24",
					Commands:     5,
					InputTokens:  10000,
					OutputTokens: 2000,
					SavedTokens:  8000,
					AvgSavings:   40.0,
				},
			}, nil
		},
	}

	// 2. Act: Ejecutar la función aislando el I/O
	args := []string{"--daily"}
	err := RunGain(mock, args)

	// 3. Assert: Verificar que no haya errores.
	// Al utilizar un mock, garantizamos que no se usa SQLite.
	if err != nil {
		t.Fatalf("No se esperaba error, se obtuvo: %v", err)
	}
}
