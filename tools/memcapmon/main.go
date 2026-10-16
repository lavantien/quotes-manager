//go:build windows

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const defaultMatch = `quotes-manager|qm[.]exe|mutation-artifacts|mutator|(^|[\\/])(go|compile[0-9a-z]*|link|cgo|gcc|g\+\+|make)(\.exe)?$|[.]test[.]exe$`

type procInfo struct {
	pid   uint32
	ppid  uint32
	name  string
	cmd   string
	ws    uint64
	valid bool
}

func main() {
	log.SetFlags(log.Ltime)
	capStr := flag.String("cap", "8GB", "combined working-set cap: plain bytes or <num><B|KB|MB|GB|KiB|MiB|GiB>")
	poll := flag.Duration("poll", 2*time.Second, "sampling interval")
	runFor := flag.Duration("for", 0, "stop after this duration, 0 means until killed")
	match := flag.String("match", defaultMatch, "regex over process name or command line")
	flag.Parse()

	limit, err := parseCap(*capStr)
	if err != nil {
		log.Fatalf("parse -cap %q: %v", *capStr, err)
	}
	re, err := regexp.Compile(*match)
	if err != nil {
		log.Fatalf("parse -match: %v", err)
	}
	deadline := time.Time{}
	if *runFor > 0 {
		deadline = time.Now().Add(*runFor)
	}
	log.Printf("memcapmon: cap %d bytes, poll %v, match %q", limit, *poll, *match)
	for {
		if !deadline.IsZero() && time.Now().After(deadline) {
			log.Printf("memcapmon: deadline reached, exiting")
			return
		}
		snap, err := snapshot()
		if err != nil {
			log.Printf("snapshot: %v", err)
			time.Sleep(*poll)
			continue
		}
		self := protectedSet(snap)
		var hits []procInfo
		var total uint64
		for _, p := range snap {
			if self[p.pid] || !shouldMatch(*p, re) {
				continue
			}
			hits = append(hits, *p)
			total += p.ws
		}
		if total > limit {
			offender := pickLargest(hits)
			log.Printf("cap breach: total %d over %d, killing pid %d %s (%d bytes)",
				total, limit, offender.pid, offender.name, offender.ws)
			killed := killTree(snap, offender.pid)
			time.Sleep(500 * time.Millisecond)
			after, err := snapshot()
			if err == nil {
				for _, pid := range killed {
					if after[pid] != nil {
						log.Printf("SURVIVOR: pid %d still present after kill", pid)
					}
				}
			}
			log.Printf("killed %d processes in offender tree", len(killed))
		}
		time.Sleep(*poll)
	}
}

func parseCap(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty cap")
	}
	units := []struct {
		suffix string
		bytes  uint64
	}{
		{"GiB", 1 << 30},
		{"MiB", 1 << 20},
		{"KiB", 1 << 10},
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
		{"B", 1},
	}
	for _, u := range units {
		if mag, ok := strings.CutSuffix(s, u.suffix); ok {
			n, err := strconv.ParseUint(strings.TrimSpace(mag), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("bad magnitude in %q", s)
			}
			return n * u.bytes, nil
		}
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad cap %q", s)
	}
	return n, nil
}

func shouldMatch(p procInfo, re *regexp.Regexp) bool {
	return re.MatchString(p.name) || (p.cmd != "" && re.MatchString(p.cmd))
}

func pickLargest(hits []procInfo) procInfo {
	if len(hits) == 0 {
		return procInfo{}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].ws != hits[j].ws {
			return hits[i].ws > hits[j].ws
		}
		return hits[i].pid < hits[j].pid
	})
	return hits[0]
}

func protectedSet(snap map[uint32]*procInfo) map[uint32]bool {
	return protectedSetWithSelf(snap, uint32(os.Getpid()))
}

func protectedSetWithSelf(snap map[uint32]*procInfo, selfPid uint32) map[uint32]bool {
	self := map[uint32]bool{selfPid: true}
	cur, ok := snap[selfPid]
	for ok && !self[cur.ppid] {
		self[cur.ppid] = true
		cur, ok = snap[cur.ppid]
	}
	return self
}

type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

var procGetProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

func snapshot() (map[uint32]*procInfo, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = windows.CloseHandle(snap) }()
	out := map[uint32]*procInfo{}
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := windows.Process32First(snap, &e); err != nil {
		return nil, err
	}
	for {
		p := &procInfo{pid: e.ProcessID, ppid: e.ParentProcessID, name: windows.UTF16ToString(e.ExeFile[:])}
		if h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, e.ProcessID); err == nil {
			var pmc processMemoryCounters
			pmc.cb = uint32(unsafe.Sizeof(pmc))
			ret, _, _ := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.cb))
			if ret != 0 {
				p.ws = uint64(pmc.workingSetSize)
				p.valid = true
			}
			_ = windows.CloseHandle(h)
		}
		out[e.ProcessID] = p
		e.Size = uint32(unsafe.Sizeof(e))
		if err := windows.Process32Next(snap, &e); err != nil {
			break
		}
	}
	return out, nil
}

func killTree(snap map[uint32]*procInfo, root uint32) []uint32 {
	children := map[uint32][]uint32{}
	for _, p := range snap {
		children[p.ppid] = append(children[p.ppid], p.pid)
	}
	var order []uint32
	var walk func(pid uint32)
	walk = func(pid uint32) {
		order = append(order, pid)
		for _, c := range children[pid] {
			walk(c)
		}
	}
	walk(root)
	for _, pid := range slices.Backward(order) {
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
		if err != nil {
			continue
		}
		_ = windows.TerminateProcess(h, 1)
		_ = windows.CloseHandle(h)
	}
	return order
}
