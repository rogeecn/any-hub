package cache

import (
	"context"
	"errors"
	"io"
	"time"
)

// Store 负责管理磁盘缓存的读写。磁盘布局遵循：
//
//	<StoragePath>/<HubName>/<path>.body    # 实际正文
//
// 每个条目仅由正文文件组成，文件的 ModTime/Size 由文件系统提供。
type Store interface {
	// Get 返回一个可流式读取的缓存条目。若不存在则返回 ErrNotFound。
	Get(ctx context.Context, locator Locator) (*ReadResult, error)

	// Put 将上游响应写入缓存，并产出新的 Entry 描述。实现需通过临时文件 + rename
	// 保证写入原子性，并在失败时清理临时文件。可选地根据 opts.ModTime 设置文件时间戳。
	Put(ctx context.Context, locator Locator, body io.Reader, opts PutOptions) (*Entry, error)

	// Remove 删除正文文件，通常用于上游错误或复合策略清理。
	Remove(ctx context.Context, locator Locator) error
}

// PutOptions 控制写入过程中的可选属性。
type PutOptions struct {
	ModTime time.Time
}

// Locator 唯一定位一个缓存条目（Hub + 相对路径），所有路径均为 URL 路径风格。
type Locator struct {
	HubName string
	Path    string
}

// Entry 表示一次缓存命中结果，包含绝对文件路径及文件信息。
type Entry struct {
	Locator   Locator `json:"locator"`
	FilePath  string  `json:"file_path"`
	SizeBytes int64   `json:"size_bytes"`
	ModTime   time.Time
}

// ReadResult 组合 Entry 与正文 Reader，便于代理层直接将 Body 流式返回。
type ReadResult struct {
	Entry  Entry
	Reader io.ReadSeekCloser
}

// ErrNotFound 表示缓存不存在。
var ErrNotFound = errors.New("cache entry not found")
