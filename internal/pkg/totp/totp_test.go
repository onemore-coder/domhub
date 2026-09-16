package totp

import (
	"testing"
	"time"
)

// RFC 6238 附录 B 测试向量（SHA1，密钥为 ASCII "12345678901234567890"）。
// 官方向量是 8 位（如 T=59 → 94287082），本项目为 6 位，取其末 6 位 287082。
func TestRFC6238Vectors(t *testing.T) {
	// "12345678901234567890" 的 Base32
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		got, err := Code(secret, time.Unix(c.unix, 0))
		if err != nil {
			t.Fatalf("T=%d: %v", c.unix, err)
		}
		if got != c.want {
			t.Errorf("T=%d: got %s, want %s", c.unix, got, c.want)
		}
	}
}

func TestVerifyWindow(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	now := time.Now()
	code, _ := Code(secret, now)
	if !Verify(secret, code) {
		t.Error("当前时刻验证码应通过")
	}
	prev, _ := Code(secret, now.Add(-30*time.Second))
	if !Verify(secret, prev) {
		t.Error("上一窗口验证码应通过（±1 容错）")
	}
	if Verify(secret, "000000x") || Verify(secret, "12345") {
		t.Error("非法格式应拒绝")
	}
}
