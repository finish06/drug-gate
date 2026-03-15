//go:build !linux

package metrics

// ProcfsSource is a stub for non-Linux platforms.
// The actual implementation lives in system_procfs.go (linux only).
type ProcfsSource struct{}

// NewProcfsSource returns a stub source on non-Linux platforms.
func NewProcfsSource() *ProcfsSource {
	return &ProcfsSource{}
}

func (s *ProcfsSource) CPUUsage() (float64, float64, error) {
	return 0, 0, nil
}

func (s *ProcfsSource) MemoryInfo() (*MemInfo, error) {
	return &MemInfo{}, nil
}

func (s *ProcfsSource) DiskUsage(_ string) (*DiskInfo, error) {
	return &DiskInfo{}, nil
}

func (s *ProcfsSource) NetworkStats() ([]NetStat, error) {
	return nil, nil
}
