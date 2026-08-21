package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// secretary-tui — 「秘書の朝刊」ダッシュボード。
// xops投稿キュー・RAG研究・ローカルLLM worker・任意のgovernance snapshotを1画面にまとめる。
// 読み取り専用。何も書き換えない。

type worker struct {
	alias, status, backend, host, desc string
}

type spoolStats struct {
	queued, sending, posted, failed int
	lastSentAt                      string
}

type model struct {
	home             string
	spool            spoolStats
	researchN        int
	workers          []worker
	governancePath   string
	governance       governanceSnapshot
	decisionCardPath string
	decisionCard     decisionCardSnapshot
	lastRefresh      time.Time
	err              string
}

type tickMsg time.Time
type refreshMsg struct {
	spool        spoolStats
	researchN    int
	workers      []worker
	governance   governanceSnapshot
	decisionCard decisionCardSnapshot
	err          string
}

type cliOptions struct {
	dump             bool
	snapshotJSON     bool
	governancePath   string
	decisionCardPath string
}

func tickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func refreshCmd(home, governancePath, decisionCardPath string) tea.Cmd {
	return func() tea.Msg {
		s, err1 := readSpoolStats(home)
		n, err2 := countResearch(home)
		w, err3 := readWorkers()
		var governance governanceSnapshot
		var err4 error
		if governancePath != "" {
			governance, err4 = readGovernance(governancePath)
		}
		var decisionCard decisionCardSnapshot
		var err5 error
		if decisionCardPath != "" {
			decisionCard, err5 = readDecisionCard(decisionCardPath)
		}
		errMsg := ""
		for _, e := range []error{err1, err2, err3, err4, err5} {
			if e != nil {
				errMsg += e.Error() + "; "
			}
		}
		return refreshMsg{spool: s, researchN: n, workers: w, governance: governance, decisionCard: decisionCard, err: errMsg}
	}
}

func readSpoolStats(home string) (spoolStats, error) {
	base := filepath.Join(home, "Workspace/Projects/Umeboshi/xops/spool")
	var s spoolStats
	count := func(dir string) int {
		entries, err := os.ReadDir(filepath.Join(base, dir))
		if err != nil {
			return 0
		}
		n := 0
		for _, e := range entries {
			if !e.IsDir() {
				n++
			}
		}
		return n
	}
	s.queued = count("queued")
	s.sending = count("sending")
	s.posted = count("posted")
	s.failed = count("failed")

	statePath := filepath.Join(base, "state.json")
	f, err := os.Open(statePath)
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "\"last_sent_at\"") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					v := strings.TrimSpace(parts[1])
					v = strings.Trim(v, ", ")
					v = strings.Trim(v, "\"")
					s.lastSentAt = v
				}
				break
			}
		}
	}
	return s, nil
}

func countResearch(home string) (int, error) {
	dir := filepath.Join(home, "Workspace/RAG/active/research")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			n++
		}
	}
	return n, nil
}

func readWorkers() ([]worker, error) {
	home, _ := os.UserHomeDir()
	script := filepath.Join(home, "Workspace/scripts/llm-seat.sh")
	if _, err := os.Stat(script); err != nil {
		return nil, err
	}
	out, err := exec.Command(script, "list").Output()
	if err != nil {
		return nil, err
	}
	var ws []worker
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 4 {
			continue
		}
		w := worker{alias: parts[0], status: parts[1], backend: parts[2], host: parts[3]}
		if len(parts) == 5 {
			w.desc = parts[4]
		}
		ws = append(ws, w)
	}
	return ws, nil
}

func initialModel(governancePath string) model {
	return initialModelWithDecisionCard(governancePath, "")
}

func initialModelWithDecisionCard(governancePath, decisionCardPath string) model {
	home, _ := os.UserHomeDir()
	return model{home: home, governancePath: governancePath, decisionCardPath: decisionCardPath}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(refreshCmd(m.home, m.governancePath, m.decisionCardPath), tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			return m, refreshCmd(m.home, m.governancePath, m.decisionCardPath)
		}
	case tickMsg:
		return m, tea.Batch(refreshCmd(m.home, m.governancePath, m.decisionCardPath), tickCmd())
	case refreshMsg:
		m.spool = msg.spool
		m.researchN = msg.researchN
		m.workers = msg.workers
		m.governance = msg.governance
		m.decisionCard = msg.decisionCard
		m.err = msg.err
		m.lastRefresh = time.Now()
	}
	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1).Margin(0, 1, 1, 0)
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("秘書の朝刊") + dimStyle.Render("  — 読み取り専用ダッシュボード（q終了 / r更新）") + "\n\n")

	// xops box
	xopsLines := fmt.Sprintf(
		"%s %d\n%s %d\n%s %d\n%s %s\n%s %s",
		labelStyle.Render("queued :"), m.spool.queued,
		labelStyle.Render("sending:"), m.spool.sending,
		labelStyle.Render("posted :"), m.spool.posted,
		labelStyle.Render("failed :"), failedStyled(m.spool.failed),
		labelStyle.Render("last   :"), m.spool.lastSentAt,
	)
	xopsBox := boxStyle.Render(titleStyle.Render("xops spool") + "\n" + xopsLines)

	// research box
	researchBox := boxStyle.Render(titleStyle.Render("RAG research") + "\n" +
		fmt.Sprintf("%s %d 件", labelStyle.Render("記事数:"), m.researchN))

	// worker box
	var wLines []string
	for _, w := range m.workers {
		dot := okStyle.Render("●")
		if w.status != "ready" {
			dot = warnStyle.Render("●")
		}
		wLines = append(wLines, fmt.Sprintf("%s %-20s %s/%s", dot, w.alias, w.backend, w.host))
	}
	if len(wLines) == 0 {
		wLines = append(wLines, dimStyle.Render("(worker一覧を取得できませんでした)"))
	}
	workerBox := boxStyle.Render(titleStyle.Render("local LLM workers") + "\n" + strings.Join(wLines, "\n"))

	top := lipgloss.JoinHorizontal(lipgloss.Top, xopsBox, researchBox)
	b.WriteString(top + "\n")
	b.WriteString(workerBox + "\n")

	if m.governancePath != "" {
		governanceBox := boxStyle.Render(titleStyle.Render("AI governance") + "\n" +
			strings.Join(governanceLines(m.governance), "\n"))
		b.WriteString(governanceBox + "\n")
	}
	if m.decisionCardPath != "" {
		decisionCardBox := boxStyle.Render(titleStyle.Render("Decision Card") + "\n" +
			strings.Join(decisionCardLines(m.decisionCard), "\n"))
		b.WriteString(decisionCardBox + "\n")
	}

	if m.err != "" {
		b.WriteString(warnStyle.Render("warnings: "+m.err) + "\n")
	}
	if !m.lastRefresh.IsZero() {
		b.WriteString(dimStyle.Render("最終更新 " + m.lastRefresh.Format("15:04:05")))
	}
	return b.String()
}

func failedStyled(n int) string {
	if n > 0 {
		return warnStyle.Render(fmt.Sprintf("%d", n))
	}
	return okStyle.Render(fmt.Sprintf("%d", n))
}

func parseOptions(args []string) (cliOptions, error) {
	flags := flag.NewFlagSet("secretary-tui", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dump := flags.Bool("dump", false, "render once as plain text")
	snapshotJSON := flags.Bool("snapshot-json", false, "emit one observation snapshot as JSON")
	governancePath := flags.String("governance", "", "read one WGM handoff or Router manifest")
	decisionCardPath := flags.String("decision-card", "", "read one explicit decision-card.v0 JSON")
	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if flags.NArg() != 0 {
		return cliOptions{}, fmt.Errorf("unexpected positional arguments")
	}
	if *governancePath != "" && *decisionCardPath != "" {
		return cliOptions{}, fmt.Errorf("governance and decision-card inputs are mutually exclusive")
	}
	if *snapshotJSON && (*dump || *governancePath == "" || *decisionCardPath != "") {
		return cliOptions{}, fmt.Errorf("snapshot-json requires only one governance input")
	}
	return cliOptions{dump: *dump, snapshotJSON: *snapshotJSON, governancePath: *governancePath, decisionCardPath: *decisionCardPath}, nil
}

func run(args []string, stdout, stderr io.Writer) int {
	options, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, "argument error: invalid arguments")
		return 2
	}
	if options.snapshotJSON {
		snapshot, err := readGovernance(options.governancePath)
		if err != nil {
			fmt.Fprintln(stderr, "snapshot_error: unable to create safe observation")
			return 2
		}
		document, err := observationSnapshot(snapshot)
		if err != nil {
			fmt.Fprintln(stderr, "snapshot_error: unable to create safe observation")
			return 2
		}
		encoded, err := json.Marshal(document)
		if err != nil {
			fmt.Fprintln(stderr, "snapshot_error: unable to encode observation")
			return 1
		}
		if _, err := fmt.Fprintln(stdout, string(encoded)); err != nil {
			fmt.Fprintln(stderr, "output_error: unable to write observation")
			return 1
		}
		return 0
	}
	if options.decisionCardPath != "" {
		if _, err := readDecisionCard(options.decisionCardPath); err != nil {
			fmt.Fprintln(stderr, "decision_card_error: unable to render supplied decision card")
			return 2
		}
	}
	if options.dump {
		m := initialModelWithDecisionCard(options.governancePath, options.decisionCardPath)
		msg := refreshCmd(m.home, m.governancePath, m.decisionCardPath)()
		newM, _ := m.Update(msg)
		fmt.Fprintln(stdout, newM.View())
		return 0
	}
	p := tea.NewProgram(initialModelWithDecisionCard(options.governancePath, options.decisionCardPath), tea.WithOutput(stdout))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(stderr, "error: dashboard failed")
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
