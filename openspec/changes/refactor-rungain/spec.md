# 📄 SDD Spec: Refactor `RunGain` con Inversión de Dependencias (DIP)

## 1. 🎯 Objetivo
Desacoplar la función `RunGain` de la implementación concreta de SQLite (`telemetry.Tracker`) introduciendo una abstracción (interfaz). Esto permitirá pruebas unitarias aisladas (Agentic TDD) sin necesidad de interactuar con la base de datos real.

## 2. 🏛️ Diseño de Interfaz (`TelemetryProvider`)
Se identificaron los siguientes métodos utilizados por `RunGain` y sus funciones auxiliares dentro del paquete `display`. Definiremos la siguiente interfaz en el paquete `display`:

```go
package display

import (
	"context"
	"github.com/Rogercode97/scouter/internal/telemetry"
)

// TelemetryProvider abstrae el acceso a las métricas de telemetría.
// Permite aislar la lógica de presentación de la base de datos (DIP).
type TelemetryProvider interface {
	GetSummary(ctx context.Context) (*telemetry.Summary, error)
	GetByCommand(ctx context.Context, limit int) ([]telemetry.CommandStat, error)
	GetDaily(ctx context.Context, days int) ([]telemetry.DailyStat, error)
	GetWeekly(ctx context.Context, weeks int) ([]telemetry.PeriodStats, error)
	GetMonthly(ctx context.Context, months int) ([]telemetry.PeriodStats, error)
	GetRecent(ctx context.Context, limit int) ([]telemetry.Record, error)
}
```

*Nota: La firma de `RunGain` debe cambiar de `func RunGain(tracker *telemetry.Tracker, args []string) error` a `func RunGain(tracker TelemetryProvider, args []string) error`, al igual que las firmas de las funciones auxiliares que reciben el tracker (`exportJSON`, `exportCSV`, `showHistory`, `showByCommand`, `showSparkline`, `showDailyReport`, `showPeriodReport`).*

## 3. 🧪 Suite de Pruebas (Agentic TDD)
Prueba unitaria utilizando un Mock basado en la interfaz `TelemetryProvider`. Se verificará que el comportamiento para la bandera `--daily` funcione correctamente sin tocar una base de datos real.

**Archivo propuesto:** `/data/data/com.termux/files/home/scouter/internal/display/gain_test.go`

```go
package display

import (
	"context"
	"testing"

	"github.com/Rogercode97/scouter/internal/telemetry"
)

// MockTelemetryProvider implementa la interfaz TelemetryProvider para pruebas.
type MockTelemetryProvider struct {
	GetSummaryFunc   func(ctx context.Context) (*telemetry.Summary, error)
	GetDailyFunc     func(ctx context.Context, days int) ([]telemetry.DailyStat, error)
	GetByCommandFunc func(ctx context.Context, limit int) ([]telemetry.CommandStat, error)
	GetWeeklyFunc    func(ctx context.Context, weeks int) ([]telemetry.PeriodStats, error)
	GetMonthlyFunc   func(ctx context.Context, months int) ([]telemetry.PeriodStats, error)
	GetRecentFunc    func(ctx context.Context, limit int) ([]telemetry.Record, error)
}

func (m *MockTelemetryProvider) GetSummary(ctx context.Context) (*telemetry.Summary, error) {
	if m.GetSummaryFunc != nil {
		return m.GetSummaryFunc(ctx)
	}
	return &telemetry.Summary{}, nil
}

func (m *MockTelemetryProvider) GetDaily(ctx context.Context, days int) ([]telemetry.DailyStat, error) {
	if m.GetDailyFunc != nil {
		return m.GetDailyFunc(ctx, days)
	}
	return []telemetry.DailyStat{}, nil
}

func (m *MockTelemetryProvider) GetByCommand(ctx context.Context, limit int) ([]telemetry.CommandStat, error) {
	if m.GetByCommandFunc != nil {
		return m.GetByCommandFunc(ctx, limit)
	}
	return []telemetry.CommandStat{}, nil
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

func (m *MockTelemetryProvider) GetRecent(ctx context.Context, limit int) ([]telemetry.Record, error) {
	if m.GetRecentFunc != nil {
		return m.GetRecentFunc(ctx, limit)
	}
	return []telemetry.Record{}, nil
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
		GetDailyFunc: func(ctx context.Context, days int) ([]telemetry.DailyStat, error) {
			if days != 7 {
				t.Errorf("Se esperaba que solicitara 7 dias por defecto si no se pasa parametro o 7 por inicializacion, se solicitó: %d", days)
			}
			return []telemetry.DailyStat{
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
```
