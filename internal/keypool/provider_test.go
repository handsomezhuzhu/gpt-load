package keypool

import (
	"errors"
	"fmt"
	"gpt-load/internal/config"
	"gpt-load/internal/encryption"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"sync"
	"testing"
)

// newTestProvider builds a KeyProvider backed by an in-memory store (no DB needed
// for SelectKey, which only reads from the store).
func newTestProvider(t *testing.T) *KeyProvider {
	t.Helper()

	svc, err := encryption.NewService("")
	if err != nil {
		t.Fatalf("failed to create noop encryption service: %v", err)
	}

	return NewProvider(nil, store.NewMemoryStore(), config.NewSystemSettingsManager(), svc)
}

// seedGroup directly seeds the store with keys and their active-list entries.
// keys must be ordered: the first key becomes the head of the active list.
func seedGroup(t *testing.T, p *KeyProvider, groupID uint, idsWithLimits map[uint]int64, orderedIDs ...uint) {
	t.Helper()

	activeKeysListKey := fmt.Sprintf("group:%d:active_keys", groupID)
	args := make([]any, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		args = append(args, id)
	}
	if err := p.store.LPush(activeKeysListKey, args...); err != nil {
		t.Fatalf("failed to seed active list: %v", err)
	}

	for id, limit := range idsWithLimits {
		keyHashKey := fmt.Sprintf("key:%d", id)
		if err := p.store.HSet(keyHashKey, map[string]any{
			"key_string":        fmt.Sprintf("sk-test-%d", id),
			"status":            models.KeyStatusActive,
			"failure_count":     0,
			"group_id":          groupID,
			"created_at":        0,
			"cooldown_until":    0,
			"concurrency_limit": limit,
		}); err != nil {
			t.Fatalf("failed to seed key %d: %v", id, err)
		}
	}
}

func TestSelectKeyFillFirstSkipsSaturatedKeys(t *testing.T) {
	p := newTestProvider(t)
	const groupID = 1
	// 队头是 key 1（limit=1），key 2 也是 limit=1
	seedGroup(t, p, groupID, map[uint]int64{1: 1, 2: 1}, 1, 2)

	// 占满 key 1 的并发槽位
	key1, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}
	if key1.ID != 1 {
		t.Fatalf("first selection = key %d, want key 1", key1.ID)
	}

	// key 1 已饱和，fill-first 应溢出到 key 2
	key2, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}
	if key2.ID != 2 {
		t.Fatalf("second selection = key %d, want key 2", key2.ID)
	}

	// 释放 key 1 的槽位后，fill-first 应重新回到队头 key 1
	p.ReleaseKey(1)
	keyAgain, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}
	if keyAgain.ID != 1 {
		t.Fatalf("selection after release = key %d, want key 1", keyAgain.ID)
	}

	p.ReleaseKey(2)
	p.ReleaseKey(1)
}

func TestSelectKeyRoundRobinSkipsSaturatedKeys(t *testing.T) {
	p := newTestProvider(t)
	const groupID = 1
	// key 1 无限制（智能策略），key 2 有上限 limit=1
	seedGroup(t, p, groupID, map[uint]int64{1: 0, 2: 1}, 1, 2)

	// 占满 key 2 的槽位（先选中它并占用）
	first, err := p.SelectKey(groupID, models.KeySelectionStrategyRoundRobin, 0)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}
	_ = first // 已占用槽位（无论哪个 key 都被占用）

	// 无论怎么轮换，饱和的 key 2 都不应被选中
	for i := 0; i < 5; i++ {
		k, err := p.SelectKey(groupID, models.KeySelectionStrategyRoundRobin, 0)
		if err != nil {
			t.Fatalf("SelectKey failed: %v", err)
		}
		if k.ID == 2 {
			t.Fatalf("saturated key 2 was selected on iteration %d", i)
		}
		p.ReleaseKey(k.ID)
	}
	p.ReleaseKey(first.ID)
}

func TestSelectKeyAllKeysBusy(t *testing.T) {
	p := newTestProvider(t)
	const groupID = 1
	seedGroup(t, p, groupID, map[uint]int64{1: 1, 2: 1}, 1, 2)

	// 占满两个 key
	k1, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}
	k2, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}

	// 全部饱和 → ErrAllKeysBusy
	_, err = p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
	if !errors.Is(err, app_errors.ErrAllKeysBusy) {
		t.Fatalf("SelectKey = %v, want ErrAllKeysBusy", err)
	}

	// 释放一个槽位后恢复可选
	p.ReleaseKey(k1.ID)
	recovered, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
	if err != nil {
		t.Fatalf("SelectKey after release failed: %v", err)
	}
	if recovered.ID != 1 && recovered.ID != 2 {
		t.Fatalf("unexpected key %d selected", recovered.ID)
	}
	p.ReleaseKey(k2.ID)
	p.ReleaseKey(recovered.ID)
}

func TestSelectKeySmartStrategyKeyAlwaysAvailable(t *testing.T) {
	p := newTestProvider(t)
	const groupID = 1
	// key 1 无限制（智能策略），key 2 有上限
	seedGroup(t, p, groupID, map[uint]int64{1: 0, 2: 1}, 1, 2)

	// 占用 key 2 的槽位
	_, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}

	// key 1 无限流，只要它在列表中，就永远不会出现 all-busy
	for i := 0; i < 3; i++ {
		k, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
		if err != nil {
			t.Fatalf("SelectKey failed: %v", err)
		}
		if k.ID == 2 {
			t.Fatalf("saturated key 2 was selected")
		}
		p.ReleaseKey(k.ID)
	}
}

func TestReleaseKeyFloorAtZero(t *testing.T) {
	p := newTestProvider(t)

	// 释放不存在的 key 不应 panic
	p.ReleaseKey(999)

	// 占用/释放后计数不会变成负数
	if !p.acquireSlot(1, 2) {
		t.Fatalf("acquireSlot failed")
	}
	if !p.acquireSlot(1, 2) {
		t.Fatalf("acquireSlot failed")
	}
	if p.acquireSlot(1, 2) {
		t.Fatalf("acquireSlot should fail when saturated")
	}
	p.ReleaseKey(1)
	p.ReleaseKey(1)
	// 过度释放不应把计数变成负数
	p.ReleaseKey(1)
	p.ReleaseKey(1)
	if got := p.InFlightCount(1); got != 0 {
		t.Fatalf("InFlightCount = %d, want 0", got)
	}
}

func TestSelectKeyGroupDefaultLimit(t *testing.T) {
	p := newTestProvider(t)
	const groupID = 1
	// key 1 未单独设置（0），key 2 单独设置了 limit=1
	seedGroup(t, p, groupID, map[uint]int64{1: 0, 2: 1}, 1, 2)

	// 组级默认 limit=2：key 1 使用组级值 2，key 2 用自己的 1
	k1, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 2)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}
	if k1.ID != 1 {
		t.Fatalf("first selection = key %d, want key 1", k1.ID)
	}

	// key 1 未满（1/2），fill-first 继续用 key 1
	k1Again, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 2)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}
	if k1Again.ID != 1 {
		t.Fatalf("second selection = key %d, want key 1 (still under group limit)", k1Again.ID)
	}

	// key 1 现在 2/2 满，key 2 用自身 limit=1，占满后全部饱和 → ErrAllKeysBusy
	k2, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 2)
	if err != nil {
		t.Fatalf("SelectKey failed: %v", err)
	}
	if k2.ID != 2 {
		t.Fatalf("third selection = key %d, want key 2", k2.ID)
	}
	_, err = p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 2)
	if !errors.Is(err, app_errors.ErrAllKeysBusy) {
		t.Fatalf("SelectKey = %v, want ErrAllKeysBusy", err)
	}

	p.ReleaseKey(1)
	p.ReleaseKey(1)
	p.ReleaseKey(2)
}

func TestSelectKeyGroupDefaultLimitZeroMeansUnset(t *testing.T) {
	p := newTestProvider(t)
	const groupID = 1
	// key 1 未单独设置，组级默认 0 = 不设置 → 智能策略，永不饱和
	seedGroup(t, p, groupID, map[uint]int64{1: 0}, 1)

	for i := 0; i < 5; i++ {
		k, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
		if err != nil {
			t.Fatalf("SelectKey failed: %v", err)
		}
		p.ReleaseKey(k.ID)
	}
}

func TestSelectKeyConcurrentAcquisition(t *testing.T) {
	p := newTestProvider(t)
	const groupID = 1
	// 单个 key，limit=5
	seedGroup(t, p, groupID, map[uint]int64{1: 5}, 1)

	// 并发抢占：100 个 goroutine 同时请求，最多 5 个成功占用，其余获得 ErrAllKeysBusy
	const total = 100
	successes := make(chan int64, total)
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k, err := p.SelectKey(groupID, models.KeySelectionStrategyFillFirst, 0)
			if err == nil {
				successes <- int64(k.ID)
			}
		}()
	}
	wg.Wait()
	close(successes)

	got := 0
	for range successes {
		got++
	}
	if got != 5 {
		t.Fatalf("concurrent acquisitions = %d, want exactly 5 (limit)", got)
	}
}
