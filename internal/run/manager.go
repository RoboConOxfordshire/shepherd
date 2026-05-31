package run

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type Manager struct {
	mu sync.Mutex

	cfg    Config
	state  State
	zone   string
	mode   Mode
	cmd    *exec.Cmd
	cmdWait chan error

	reaperTimer *time.Timer
	reapTime    time.Time
	fifoDir     string
	fifoPath    string
	outputFile  *os.File
}

type Config struct {
	UserCodeEntrypoint string
	UserCodePath       string
	RobotLibPath       string
	OutputFilePath     string
	RoundLength        time.Duration
	ReapGracePeriod    time.Duration
}

func NewManager(cfg Config) (*Manager, error) {
	m := &Manager{
		cfg:   cfg,
		state: StateReady,
	}

	if err := m.createFIFO(); err != nil {
		return nil, fmt.Errorf("create fifo: %w", err)
	}

	return m, nil
}

func (m *Manager) createFIFO() error {
	dir, err := os.MkdirTemp("", "shepherd-fifo-")
	if err != nil {
		return err
	}
	m.fifoDir = dir
	m.fifoPath = filepath.Join(dir, "start.fifo")
	return syscall.Mkfifo(m.fifoPath, 0666)
}

func (m *Manager) StartUserCode() error {
	m.mu.Lock()
	if m.state == StateRunning {
		m.mu.Unlock()
		return fmt.Errorf("code is already running")
	}
	if m.cmdWait != nil {
		select {
		case <-m.cmdWait:
		default:
		}
	}
	m.mu.Unlock()

	entrypoint := filepath.Join(m.cfg.UserCodePath, m.cfg.UserCodeEntrypoint)

	if err := os.MkdirAll(filepath.Dir(m.cfg.OutputFilePath), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	f, err := os.Create(m.cfg.OutputFilePath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}

	cmd := exec.Command("python3", "-u", entrypoint, "--startfifo", m.fifoPath)
	cmd.Dir = m.cfg.UserCodePath
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+m.cfg.RobotLibPath,
	)

	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("start user code: %w", err)
	}

	cmdWait := make(chan error, 1)
	go m.waitForExit(cmd, cmdWait)

	m.mu.Lock()
	m.cmd = cmd
	m.cmdWait = cmdWait
	m.outputFile = f
	m.state = StateReady
	m.mu.Unlock()

	return nil
}

func (m *Manager) waitForExit(cmd *exec.Cmd, cmdWait chan<- error) {
	err := cmd.Wait()
	cmdWait <- err

	m.mu.Lock()
	if err != nil {
		log.Printf("user code exited: %v", err)
	}
	if m.state == StateRunning {
		m.state = StatePostRun
	}
	m.mu.Unlock()
}

func (m *Manager) SendStartSignal(zone string, mode Mode) error {
	m.mu.Lock()
	m.zone = zone
	m.mode = mode
	m.state = StateRunning
	m.mu.Unlock()

	data := map[string]any{
		"mode":  string(mode),
		"zone":  zone,
		"arena": "A",
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal start data: %w", err)
	}

	return os.WriteFile(m.fifoPath, raw, 0644)
}

func (m *Manager) StartReaper() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reapTime = time.Now().Add(m.cfg.RoundLength)
	m.reaperTimer = time.AfterFunc(m.cfg.RoundLength, func() {
		m.Reap("end of round")
	})
}

func (m *Manager) StopReaper() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.reaperTimer != nil {
		m.reaperTimer.Stop()
		m.reaperTimer = nil
	}
}

func (m *Manager) Reap(reason string) {
	m.mu.Lock()
	log.Printf("Reaping user code (%s)", reason)
	if m.state != StateRunning {
		log.Printf("Warning: state is %s, not running", m.state)
		m.mu.Unlock()
		return
	}
	if m.cmd == nil || m.cmd.Process == nil {
		m.mu.Unlock()
		return
	}
	cmdWait := m.cmdWait
	m.mu.Unlock()

	if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("SIGTERM failed: %v", err)
	}

	select {
	case <-cmdWait:
	case <-time.After(m.cfg.ReapGracePeriod):
		log.Println("Butchering user code")
		if err := m.cmd.Process.Signal(syscall.SIGKILL); err != nil {
			log.Printf("SIGKILL failed: %v", err)
		}
		<-cmdWait
	}

	m.mu.Lock()
	if m.outputFile != nil {
		m.outputFile.WriteString("\n==== END OF ROUND ====\n\n")
		m.outputFile.Close()
		m.outputFile = nil
	}
	m.state = StatePostRun
	m.mu.Unlock()

	log.Println("Done reaping user code")
}

func (m *Manager) Stop() {
	m.StopReaper()
	m.Reap("manual stop")
}

func (m *Manager) Reset() {
	m.mu.Lock()
	m.state = StateReady
	m.zone = ""
	m.mode = ""

	if m.outputFile != nil {
		m.outputFile.Close()
		m.outputFile = nil
	}

	if m.fifoDir != "" {
		os.RemoveAll(m.fifoDir)
		m.fifoDir = ""
		m.fifoPath = ""
	}
	m.mu.Unlock()

	m.createFIFO()

	cmd := exec.Command("python3", "-c", "import robot.reset; robot.reset.reset()")
	cmd.Env = append(os.Environ(), "PYTHONPATH="+m.cfg.RobotLibPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("robot.reset failed: %v\n%s", err, out)
	}
}

func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == StateRunning && m.cmd != nil && m.cmd.ProcessState != nil && m.cmd.ProcessState.Exited() {
		m.state = StatePostRun
	}
	return m.state
}

func (m *Manager) TimeLeft() *time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.reapTime.IsZero() {
		return nil
	}
	left := time.Until(m.reapTime)
	if left <= 0 {
		return nil
	}
	return &left
}

func (m *Manager) Zone() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.zone
}

func (m *Manager) Mode() Mode {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mode
}

func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.reaperTimer != nil {
		m.reaperTimer.Stop()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
	}
	if m.cmdWait != nil {
		select {
		case <-m.cmdWait:
		default:
		}
	}
	if m.outputFile != nil {
		m.outputFile.Close()
	}
	if m.fifoDir != "" {
		os.RemoveAll(m.fifoDir)
	}
}
