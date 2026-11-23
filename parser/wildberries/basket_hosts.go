package wildberries

import (
	"context"
	"fmt"
	"sync"
)

const basketScanWorkers = 16

// basketHostByVol: WB keeps a whole vol on one shard, so the next item from the
// same vol skips the scan.
var basketHostByVol sync.Map

func basketHosts() []string {
	hosts := make([]string, 0, basketHostMaxIndex*len(basketHostPatterns))
	for basket := 1; basket <= basketHostMaxIndex; basket++ {
		for _, pattern := range basketHostPatterns {
			hosts = append(hosts, fmt.Sprintf(pattern, basket))
		}
	}
	return hosts
}

// findBasketHost probes shards in parallel and returns the first host where try
// succeeds. try may run concurrently; only the winning call's side effects count.
func findBasketHost(ctx context.Context, vol int, try func(context.Context, string) bool) (string, bool) {
	if cached, ok := basketHostByVol.Load(vol); ok {
		if host, _ := cached.(string); try(ctx, host) {
			return host, true
		}
		basketHostByVol.Delete(vol)
	}

	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	hosts := basketHosts()
	jobs := make(chan string)
	winner := make(chan string, 1)
	var wg sync.WaitGroup
	for range basketScanWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for host := range jobs {
				if scanCtx.Err() != nil {
					continue
				}
				if try(scanCtx, host) {
					select {
					case winner <- host:
						cancel()
					default:
					}
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, host := range hosts {
			select {
			case jobs <- host:
			case <-scanCtx.Done():
				return
			}
		}
	}()
	wg.Wait()

	select {
	case host := <-winner:
		basketHostByVol.Store(vol, host)
		return host, true
	default:
		return "", false
	}
}
