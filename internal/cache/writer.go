package cache

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

// ErrStoreUnavailable 表示当前模块未注入缓存存储实例。
var ErrStoreUnavailable = errors.New("cache store unavailable")

// StrategyWriter 注入模块的缓存策略，提供 TTL 决策与写入封装。
type StrategyWriter struct {
	store    Store
	strategy hubmodule.CacheStrategyProfile
	now      func() time.Time
}

// NewStrategyWriter 构造策略感知的写入器，默认使用 time.Now 作为时钟。
func NewStrategyWriter(store Store, strategy hubmodule.CacheStrategyProfile) StrategyWriter {
	return StrategyWriter{
		store:    store,
		strategy: strategy,
		now:      time.Now,
	}
}

// Enabled 返回当前是否具备缓存写入能力。
func (w StrategyWriter) Enabled() bool {
	return w.store != nil
}

// Put 写入缓存正文，并保持与 Store 相同的语义。
func (w StrategyWriter) Put(ctx context.Context, locator Locator, body io.Reader, opts PutOptions) (*Entry, error) {
	if w.store == nil {
		return nil, ErrStoreUnavailable
	}
	return w.store.Put(ctx, locator, body, opts)
}

// ShouldBypassValidation 根据策略 TTL 判断是否可以直接复用缓存，避免重复 HEAD。
func (w StrategyWriter) ShouldBypassValidation(entry Entry) bool {
	ttl := w.strategy.TTLHint
	if ttl <= 0 {
		return false
	}
	expireAt := entry.ModTime.Add(ttl)
	return w.now().Before(expireAt)
}

// SupportsValidation 返回当前策略是否允许通过 HEAD/Etag 等方式再验证。
func (w StrategyWriter) SupportsValidation() bool {
	return w.strategy.ValidationMode != hubmodule.ValidationModeNever
}
