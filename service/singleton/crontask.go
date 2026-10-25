package singleton

import (
	"sync"

	"github.com/robfig/cron/v3"
)

// CronClass is the dashboard's internal job scheduler. The user-facing
// scheduled/trigger task feature (Cron model, command dispatch) has been
// removed; only the embedded cron.Cron remains for panel-internal recurring
// jobs (monitor history cleanup, transfer usage sampling, session GC,
// ServiceSentinel maintenance, ...).
type CronClass struct {
	*cron.Cron
	closeOnce sync.Once
}

// Close stops the scheduler and joins every job before a test restores globals.
// The embedded cron.Stop only exposes the completion context; callers must await it.
func (c *CronClass) Close() {
	if c == nil || c.Cron == nil {
		return
	}
	c.closeOnce.Do(func() {
		<-c.Cron.Stop().Done()
	})
}

func NewCronClass() *CronClass {
	cronx := cron.New(cron.WithSeconds(), cron.WithLocation(Loc))
	cronx.Start()

	return &CronClass{
		Cron: cronx,
	}
}

func userIsAdmin(userID uint64) bool {
	if userID == 0 {
		return true
	}

	UserLock.RLock()
	defer UserLock.RUnlock()

	userInfo, ok := UserInfoMap[userID]
	return ok && userInfo.Role.IsAdmin()
}
