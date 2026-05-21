package display

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/telemetry"
	"github.com/Rogercode97/scouter/internal/utils"
	"github.com/charmbracelet/lipgloss"
)

// RunGain executes the gain (token savings report) command.
func RunGain(tracker *telemetry.Tracker, args []string) error {
	if tracker == nil {
		PrintError("no tracking data (run some commands first)")
		return nil
	}

	// Parse args
	var (
		showDaily   bool
		showWeekly  bool
		showMonthly bool
		showJSON    bool
		showCSV     bool
		showTop     bool
		historyN    int
		topN        int
		days        = 7
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--daily":
			showDaily = true
		case "--weekly":
			showWeekly = true
		case "--monthly":
			showMonthly = true
		case "--json":
			showJSON = true
		case "--csv":
			showCSV = true
		case "--top":
			showTop = true
			if i+1 < len(args) {
				_, _ = fmt.Sscanf(args[i+1], "%d", &topN)
				i++
			}
			if topN <= 0 {
				topN = 10
			}
		case "--history":
			if i+1 < len(args) {
				_, _ = fmt.Sscanf(args[i+1], "%d", &historyN)
				i++
			}
			if historyN <= 0 {
				historyN = 10
			}
		}
	}

	ctx := context.Background()
	stats, err := tracker.GetGainStats(ctx)
	if err != nil {
		return fmt.Errorf("get gain stats: %w", err)
	}

	if showJSON {
		summary, _ := tracker.GetSummary(ctx)
		return exportJSON(summary, tracker, days)
	}
	if showCSV {
		return exportCSV(tracker, days)
	}

	if historyN > 0 {
		return showHistory(tracker, historyN)
	}

	if showTop {
		printDashboard(stats)
		return showByCommand(tracker, topN)
	}

	if showWeekly {
		printDashboard(stats)
		return showPeriodReport(tracker, "weekly")
	}

	if showMonthly {
		printDashboard(stats)
		return showPeriodReport(tracker, "monthly")
	}

	if showDaily {
		return showDailyReport(tracker, days, stats)
	}

	// Default: full dashboard (summary + sparkline + top commands)
	printDashboard(stats)
	showSparkline(tracker)
	_ = showByCommand(tracker, 10)
	return nil
}

func printDashboard(s *telemetry.GainStats) {
	tty := IsTerminal()

	if !tty {
		fmt.Println("\n  scouter — Token Savings Report")
		fmt.Println("  " + FormatSeparator(30))
		fmt.Printf("  %-20s  %d\n", "Commands filtered", s.TotalCommands)
		fmt.Printf("  %-20s  %s\n", "Tokens saved", utils.FormatTokens(s.TotalSaved))
		fmt.Printf("  %-20s  %.1f%%\n", "Avg savings", s.AvgSavings)
		fmt.Printf("  %-20s  %.1fh\n", "Time reclaimed", s.HoursReclaimed)
		fmt.Println()
		return
	}

	// KPI Block: Tokens Saved
	kpi1 := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(HeaderStyle.GetForeground()).
		Width(24).
		Render(fmt.Sprintf("%s\n%s", DimStyle.Render("TOKENS SAVED"), StatStyle.Render(utils.FormatTokens(s.TotalSaved))))

	// KPI Block: Time Reclaimed
	timeStr := fmt.Sprintf("%.1fh", s.HoursReclaimed)
	if s.HoursReclaimed < 1.0 {
		mins := int(s.HoursReclaimed * 60)
		if mins == 0 {
			timeStr = "< 1m"
		} else {
			timeStr = fmt.Sprintf("%dm", mins)
		}
	}
	kpi2 := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(SuccessStyle.GetForeground()).
		Width(24).
		Render(fmt.Sprintf("%s\n%s", DimStyle.Render("TIME RECLAIMED"), SuccessStyle.Bold(true).Render(timeStr)))

	// KPI Block: Efficiency
	kpi3 := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(HeaderStyle.GetForeground()).
		Width(24).
		Render(fmt.Sprintf("%s\n%s", DimStyle.Render("EFFICIENCY"), ColorTier(TierLabel(s.AvgSavings))))

	row := lipgloss.JoinHorizontal(lipgloss.Top, kpi1, kpi2, kpi3)

	fmt.Println()
	fmt.Println(HeaderStyle.Render("  scouter — Strategic Gain Dashboard"))
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().MarginLeft(2).Render(row))
	fmt.Println()
}

func showByCommand(tracker *telemetry.Tracker, limit int) error {
	stats, err := tracker.GetByCommand(context.Background(), limit)
	if err != nil {
		return err
	}
	if len(stats) == 0 {
		return nil
	}

	tty := IsTerminal()

	// Find max saved for bar scaling
	maxSaved := 0
	for _, s := range stats {
		if s.SavedTokens > maxSaved {
			maxSaved = s.SavedTokens
		}
	}

	if tty {
		fmt.Println(DimStyle.Render("  Top commands by tokens saved"))
		fmt.Println()
	} else {
		fmt.Println("  Top commands by tokens saved")
		fmt.Println()
	}

	headers := []string{"Command", "Runs", "Saved", "Savings", "Impact"}
	var rows [][]string
	for _, s := range stats {
		cmd := s.Command
		if len(cmd) > 25 {
			cmd = cmd[:22] + "..."
		}
		bar := ColorBar(s.SavedTokens, maxSaved, 12)
		rows = append(rows, []string{
			cmd,
			fmt.Sprintf("%d", s.Count),
			utils.FormatTokens(s.SavedTokens),
			ColorSavings(s.AvgSavings),
			bar,
		})
	}

	fmt.Print(FormatTable(headers, rows))
	fmt.Println()
	return nil
}

func showSparkline(tracker *telemetry.Tracker) {
	daily, err := tracker.GetDaily(context.Background(), 14)
	if err != nil || len(daily) < 2 {
		return
	}

	// Daily data is DESC, reverse for chronological sparkline
	values := make([]float64, len(daily))
	for i, d := range daily {
		values[len(daily)-1-i] = d.AvgSavings
	}

	spark := FormatSparkline(values)
	tty := IsTerminal()

	if tty {
		fmt.Printf("  %s  %s\n", DimStyle.Render("14-day trend"), SuccessStyle.Render(spark))
	} else {
		fmt.Printf("  14-day trend  %s\n", spark)
	}
	fmt.Println()
}

func showDailyReport(tracker *telemetry.Tracker, days int, stats *telemetry.GainStats) error {
	daily, err := tracker.GetDaily(context.Background(), days)
	if err != nil {
		return err
	}

	printDashboard(stats)

	headers := []string{"Date", "Cmds", "Input", "Output", "Saved", "Savings"}
	var rows [][]string
	for _, d := range daily {
		rows = append(rows, []string{
			d.Day,
			fmt.Sprintf("%d", d.Commands),
			utils.FormatTokens(d.InputTokens),
			utils.FormatTokens(d.OutputTokens),
			utils.FormatTokens(d.SavedTokens),
			ColorSavings(d.AvgSavings),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	return nil
}

func showPeriodReport(tracker *telemetry.Tracker, period string) error {
	var stats []telemetry.PeriodStats
	var err error
	var label string

	switch period {
	case "weekly":
		stats, err = tracker.GetWeekly(context.Background(), 8)
		label = "Weekly"
	case "monthly":
		stats, err = tracker.GetMonthly(context.Background(), 6)
		label = "Monthly"
	default:
		return fmt.Errorf("unknown period: %s", period)
	}
	if err != nil {
		return err
	}

	tty := IsTerminal()
	if tty {
		fmt.Println(DimStyle.Render(fmt.Sprintf("  %s breakdown", label)))
	} else {
		fmt.Printf("  %s breakdown\n", label)
	}
	fmt.Println()

	headers := []string{"Period", "Cmds", "Input", "Output", "Saved", "Savings"}
	var rows [][]string
	for _, s := range stats {
		rows = append(rows, []string{
			s.Period,
			fmt.Sprintf("%d", s.Commands),
			utils.FormatTokens(s.InputTokens),
			utils.FormatTokens(s.OutputTokens),
			utils.FormatTokens(s.SavedTokens),
			ColorSavings(s.AvgSavings),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	fmt.Println()
	return nil
}

func showHistory(tracker *telemetry.Tracker, n int) error {
	records, err := tracker.GetRecent(context.Background(), n)
	if err != nil {
		return err
	}

	headers := []string{"Command", "Input", "Output", "Saved", "Time"}
	var rows [][]string
	for _, r := range records {
		cmd := r.OriginalCmd
		if len(cmd) > 30 {
			cmd = cmd[:27] + "..."
		}
		rows = append(rows, []string{
			cmd,
			utils.FormatTokens(r.InputTokens),
			utils.FormatTokens(r.OutputTokens),
			ColorSavings(r.SavingsPct),
			fmt.Sprintf("%dms", r.ExecTimeMs),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	return nil
}

func exportJSON(summary *telemetry.Summary, tracker *telemetry.Tracker, days int) error {
	daily, _ := tracker.GetDaily(context.Background(), days)
	byCmd, _ := tracker.GetByCommand(context.Background(), 10)
	data := map[string]any{
		"summary":    summary,
		"daily":      daily,
		"by_command": byCmd,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func exportCSV(tracker *telemetry.Tracker, days int) error {
	daily, err := tracker.GetDaily(context.Background(), days)
	if err != nil {
		return err
	}

	w := csv.NewWriter(os.Stdout)
	_ = w.Write([]string{"date", "commands", "input_tokens", "output_tokens", "saved_tokens", "avg_savings"})
	for _, d := range daily {
		_ = w.Write([]string{
			d.Day,
			fmt.Sprintf("%d", d.Commands),
			fmt.Sprintf("%d", d.InputTokens),
			fmt.Sprintf("%d", d.OutputTokens),
			fmt.Sprintf("%d", d.SavedTokens),
			fmt.Sprintf("%.1f", d.AvgSavings),
		})
	}
	w.Flush()
	return w.Error()
}
