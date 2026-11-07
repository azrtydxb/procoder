package parallel

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// CPUs is how many CPUs this process may actually use: runtime.NumCPU()
// capped by the cgroup CPU quota, at least one.
//
// runtime.NumCPU() counts the host's cores (minus the affinity mask), not
// the container's quota, and go.mod's `go 1.23` keeps Go 1.25's
// container-aware GOMAXPROCS off. On a CI job pod that sees 256 cores but
// may use 16, sizing by NumCPU started hundreds of formatter processes,
// which then missed their deadline and were reported as not checked.
func CPUs() int { return cpusFrom("/sys/fs/cgroup", runtime.NumCPU()) }

// cpusFrom caps host by the quota found under the cgroup root: cgroup v2's
// cpu.max, else cgroup v1's cpu.cfs_quota_us / cpu.cfs_period_us. A missing,
// unlimited or unreadable quota leaves host as it is. A fractional quota
// rounds up, so 1.5 CPUs allows two processes.
func cpusFrom(root string, host int) int {
	host = maxInt(1, host)
	quota, period, ok := cgroupV2(root)
	if !ok {
		quota, period, ok = cgroupV1(root)
	}
	if !ok || quota <= 0 || period <= 0 {
		return host
	}
	n := int((quota + period - 1) / period)
	if n < 1 {
		n = 1
	}
	if n < host {
		return n
	}
	return host
}

func cgroupV2(root string) (quota, period int64, ok bool) {
	b, err := os.ReadFile(filepath.Join(root, "cpu.max"))
	if err != nil {
		return 0, 0, false
	}
	f := strings.Fields(string(b))
	if len(f) != 2 || f[0] == "max" {
		return 0, 0, false
	}
	return parsePair(f[0], f[1])
}

func cgroupV1(root string) (quota, period int64, ok bool) {
	for _, dir := range []string{"cpu", "cpu,cpuacct"} {
		q, err1 := os.ReadFile(filepath.Join(root, dir, "cpu.cfs_quota_us"))
		p, err2 := os.ReadFile(filepath.Join(root, dir, "cpu.cfs_period_us"))
		if err1 == nil && err2 == nil {
			return parsePair(strings.TrimSpace(string(q)), strings.TrimSpace(string(p)))
		}
	}
	return 0, 0, false
}

func parsePair(q, p string) (quota, period int64, ok bool) {
	quota, err1 := strconv.ParseInt(q, 10, 64)
	period, err2 := strconv.ParseInt(p, 10, 64)
	if err1 != nil || err2 != nil || quota <= 0 || period <= 0 {
		return 0, 0, false
	}
	return quota, period, true
}
