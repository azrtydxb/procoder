package parallel

import (
	"os"
	"path/filepath"
	"testing"
)

// A CI job container can see every core of its host but be allowed far
// fewer: the arc runners on quadronix show runtime.NumCPU() = 256 under a
// cgroup cpu.max of 16 CPUs. A budget of 256 started ~350 prettier
// processes on 16 CPUs and every one missed its 30-second deadline, so the
// gate reported the files as UNCHECKED. The budget has to follow the quota.
//
// proved by: return host from cpusFrom unchanged — the quota cases fail.
func TestCPUsFromFollowsTheCgroupQuota(t *testing.T) {
	cases := []struct {
		name   string
		cpuMax string // "" = no cgroup v2 file
		v1     [2]string
		host   int
		want   int
	}{
		{name: "quadronix runner", cpuMax: "1600000 100000\n", host: 256, want: 16},
		{name: "fractional quota rounds up", cpuMax: "150000 100000\n", host: 8, want: 2},
		{name: "unlimited", cpuMax: "max 100000\n", host: 10, want: 10},
		{name: "quota above host", cpuMax: "6400000 100000\n", host: 4, want: 4},
		{name: "no cgroup", host: 6, want: 6},
		{name: "garbage", cpuMax: "lots\n", host: 6, want: 6},
		{name: "cgroup v1 quota", v1: [2]string{"400000\n", "100000\n"}, host: 64, want: 4},
		{name: "cgroup v1 unlimited", v1: [2]string{"-1\n", "100000\n"}, host: 64, want: 64},
		{name: "tiny quota still one", cpuMax: "1000 100000\n", host: 8, want: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			if c.cpuMax != "" {
				write(t, filepath.Join(root, "cpu.max"), c.cpuMax)
			}
			if c.v1[0] != "" {
				write(t, filepath.Join(root, "cpu", "cpu.cfs_quota_us"), c.v1[0])
				write(t, filepath.Join(root, "cpu", "cpu.cfs_period_us"), c.v1[1])
			}
			if got := cpusFrom(root, c.host); got != c.want {
				t.Errorf("cpusFrom(%q, %d) = %d, want %d", c.name, c.host, got, c.want)
			}
		})
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
