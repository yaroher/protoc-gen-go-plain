package type_alias

import "testing"

func BenchmarkIntoPlain(b *testing.B) {
	msg := sampleUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = msg.IntoPlain()
	}
}

func BenchmarkIntoPlainErr(b *testing.B) {
	msg := sampleUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.IntoPlainErr()
	}
}

func BenchmarkIntoPb(b *testing.B) {
	msg := sampleUserPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = msg.IntoPb()
	}
}

func BenchmarkIntoPbErr(b *testing.B) {
	msg := sampleUserPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.IntoPbErr()
	}
}
