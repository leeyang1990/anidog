package titleparse

import "testing"

func TestInstallmentIdentity(t *testing.T) {
	for _, tc := range []struct {
		target, candidate string
		want              bool
	}{
		{"赛博朋克：边缘行者 2", "【喵萌奶茶屋】[赛博朋克：边缘行者合集1-10] [1080p]", false},
		{"赛博朋克：边缘行者 2", "赛博朋克：边缘行者 2", true},
		{"碧蓝之海 第3季", "碧蓝之海", false},
		{"碧蓝之海 第3季", "[组] 碧蓝之海 第三季 - 12 [1080p]", true},
		{"Re：从零开始的异世界生活 第四季 夺还篇", "Re：从零开始的异世界生活 第四季 丧失篇", false},
		{"Re：从零开始的异世界生活 第四季 夺还篇", "Re：从零开始的异世界生活 第四季", false},
		{"Re：从零开始的异世界生活 第四季 夺还篇", "Re：从零开始的异世界生活 第四季 奪還篇", true},
		{"幸运星", "[组] Lucky Star / 幸运星 [01-24][1080p]", true},
		{"Queue Test", "[Test] Queue Test - 02 [1080p]", true},
	} {
		if got := InstallmentCompatible(tc.target, tc.candidate); got != tc.want {
			t.Errorf("%q / %q = %v", tc.target, tc.candidate, got)
		}
	}
}
