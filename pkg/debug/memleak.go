package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sort"
	"time"

	"github.com/infrastructure-io/topohub/pkg/clients/redfish"
	"github.com/infrastructure-io/topohub/pkg/clients/ssh"
	"github.com/infrastructure-io/topohub/pkg/lock"
	"github.com/infrastructure-io/topohub/pkg/log"
)

const (
	// HeapProfileDir is the directory where heap profiles are saved.
	// Mount a hostPath or emptyDir to persist across restarts.
	HeapProfileDir = "/tmp/topohub-heap-profiles"

	// heapDumpInterval controls how often a heap profile is auto-saved.
	heapDumpInterval = 6 * time.Hour

	// diagLogInterval controls how often data-structure sizes are logged.
	diagLogInterval = 5 * time.Minute
)

// RunMemLeakDiag starts two background goroutines:
//  1. Periodic heap profile dumper (force GC → write .pb.gz file)
//  2. Periodic diagnostic logger (data-structure sizes)
//
// Call this once from main. The goroutines stop when stopCh is closed.
func RunMemLeakDiag(stopCh <-chan struct{}) {
	if err := os.MkdirAll(HeapProfileDir, 0o750); err != nil {
		log.Logger.Warnf("[mem-diag] failed to create heap profile dir %s: %v", HeapProfileDir, err)
	}

	go heapDumpLoop(stopCh)
	go diagLogLoop(stopCh)

	log.Logger.Infof("[mem-diag] memory leak diagnostics started (heap dump every %v, diag log every %v, dir=%s)",
		heapDumpInterval, diagLogInterval, HeapProfileDir)
}

// heapDumpLoop periodically forces GC and writes a heap profile to disk.
// After collecting, it cleans up old files keeping only the latest 20.
func heapDumpLoop(stopCh <-chan struct{}) {
	// Dump one right at startup as a baseline.
	dumpHeapProfile("startup")

	ticker := time.NewTicker(heapDumpInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			// Dump a final profile before exiting.
			dumpHeapProfile("shutdown")
			return
		case <-ticker.C:
			dumpHeapProfile("periodic")
		}
	}
}

func dumpHeapProfile(tag string) {
	// Force GC so the profile reflects genuinely live objects.
	runtime.GC()

	filename := fmt.Sprintf("heap_%s_%s.pb.gz", tag, time.Now().UTC().Format("20060102_150405"))
	path := filepath.Join(HeapProfileDir, filename)
	f, err := os.Create(path)
	if err != nil {
		log.Logger.Warnf("[mem-diag] failed to create heap profile %s: %v", path, err)
		return
	}
	defer f.Close()

	if err := pprof.WriteHeapProfile(f); err != nil {
		log.Logger.Warnf("[mem-diag] failed to write heap profile: %v", err)
		return
	}
	log.Logger.Infof("[mem-diag] heap profile saved: %s", path)

	cleanupOldProfiles(20)
}

func cleanupOldProfiles(keep int) {
	entries, err := os.ReadDir(HeapProfileDir)
	if err != nil {
		return
	}
	if len(entries) <= keep {
		return
	}
	// Sort by name (contains timestamp) ascending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries[:len(entries)-keep] {
		os.Remove(filepath.Join(HeapProfileDir, e.Name()))
	}
}

// diagLogLoop periodically logs sizes of all known data structures
// that could accumulate memory.
func diagLogLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(diagLogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			logDiag()
		}
	}
}

func logDiag() {
	// Force return unused memory to OS to prevent RSS from growing indefinitely
	debug.FreeOSMemory()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	redfishPoolSize := redfish.GetSessionPool().Size()
	sshPoolSize := ssh.GetSessionPool().Size()
	lockMgrSize := lock.LockManagerInstance.Size()

	log.Logger.Infof(
		"[mem-diag] goroutines=%d heapAlloc=%dKB heapInuse=%dKB heapObjects=%d "+
			"stackInuse=%dKB mspanInuse=%dKB mcacheInuse=%dKB "+
			"redfishSessions=%d sshSessions=%d lockMgrEntries=%d numGC=%d",
		runtime.NumGoroutine(),
		m.HeapAlloc/1024, m.HeapInuse/1024, m.HeapObjects,
		m.StackInuse/1024, m.MSpanInuse/1024, m.MCacheInuse/1024,
		redfishPoolSize, sshPoolSize, lockMgrSize,
		m.NumGC,
	)
}
