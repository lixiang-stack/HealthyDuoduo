// 护理事件持久化端口（care-events 票 01）；实现见 internal/storage/store_care.go。
package care

import (
	"context"
	"time"
)

// Store 护理事件读写边界；范围读取按 [from, to) 重叠语义返回，倒序 started_at DESC, id DESC。
type Store interface {
	// List 返回与该范围有重叠的事件：[from, to)，from 含、to 不含。
	List(ctx context.Context, memberID int64, from, to time.Time) ([]CareEvent, error)
	// Create 新增事件；id 由存储分配，source 固定 parent，member 不存在由外键拒绝。
	Create(ctx context.Context, memberID int64, input EventInput) (*CareEvent, error)
	// Update 完整替换可编辑内容，保留 id / member_id / source；输入合法但事件不存在返回 nil, nil。
	Update(ctx context.Context, eventID int64, input EventInput) (*CareEvent, error)
	// Delete 硬删除；事件不存在返回 false, nil。
	Delete(ctx context.Context, eventID int64) (bool, error)
}
