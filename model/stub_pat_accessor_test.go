package model

// stubPATAccessor 是只在测试里用的最小 APITokenAccessor，仅按 ids
// 字面包含判断。够用就行，不引入 *APIToken 在 model 包里转译 CSV。
type stubPATAccessor struct {
	ids []uint64
}

func (s *stubPATAccessor) CanAccessServer(id uint64) bool {
	for _, x := range s.ids {
		if x == id {
			return true
		}
	}
	return false
}

// ServerIDs 暴露白名单，使 DenyListSafeForLimitedPAT 能区分「unscoped PAT」
// 与「server-limited PAT」；缺这个方法时所有 stub 都会被当作不受限放行。
func (s *stubPATAccessor) ServerIDs() []uint64 {
	return s.ids
}
