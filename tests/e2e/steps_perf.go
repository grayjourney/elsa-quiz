//go:build e2e

package e2e

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cucumber/godog"
)

func registerPerfSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^each connected participant submits a correct answer$`, w.eachConnectedSubmitsMeasured)
	ctx.Step(`^every score update is broadcast within (\d+)ms at p95$`, w.p95Within)
	ctx.Step(`^(\d+) concurrent quiz sessions each with (\d+) participants$`, w.manySessions)
	ctx.Step(`^all participants submit answers simultaneously$`, w.allSessionsSubmit)
	ctx.Step(`^the server processes every submission without dropping any$`, w.everySubmissionProcessed)
}

func (w *World) eachConnectedSubmitsMeasured() error {
	w.latencies = w.latencies[:0]
	for _, name := range w.connected {
		d, err := w.wsSubmitMeasure(name)
		if err != nil {
			return err
		}
		w.latencies = append(w.latencies, d)
	}
	return nil
}

func (w *World) p95Within(ms int) error {
	if len(w.latencies) == 0 {
		return fmt.Errorf("no latencies measured")
	}
	ds := append([]time.Duration(nil), w.latencies...)
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	idx := (len(ds) * 95) / 100
	if idx >= len(ds) {
		idx = len(ds) - 1
	}
	p95 := ds[idx]
	budget := time.Duration(ms) * time.Millisecond
	fmt.Printf("[perf] broadcast latency p95=%v (n=%d, budget=%v)\n", p95, len(ds), budget)
	if p95 > budget {
		return fmt.Errorf("p95 latency %v exceeds %v", p95, budget)
	}
	return nil
}

func (w *World) manySessions(n, m int) error {
	if err := w.createManySessions(n, m); err != nil {
		return err
	}
	w.perfTotal = n * m
	return nil
}

func (w *World) allSessionsSubmit() error {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
		ok int
	)
	for _, sess := range w.perfSessions {
		for _, uid := range sess.users {
			wg.Add(1)
			go func(id, uid string) {
				defer wg.Done()
				st, _, err := w.rest.do("POST", "/api/sessions/"+id+"/answers", uid, submitReq{QuestionID: "Q1", Answer: correctAnswerFor("Q1")})
				if err == nil && st == 200 {
					mu.Lock()
					ok++
					mu.Unlock()
				}
			}(sess.id, uid)
		}
	}
	wg.Wait()
	w.perfAccepted = ok
	return nil
}

func (w *World) everySubmissionProcessed() error {
	fmt.Printf("[perf] throughput accepted=%d/%d submissions\n", w.perfAccepted, w.perfTotal)
	if w.perfAccepted != w.perfTotal {
		return fmt.Errorf("processed %d/%d submissions (dropped %d)", w.perfAccepted, w.perfTotal, w.perfTotal-w.perfAccepted)
	}
	return nil
}
