package override_type

import "testing"

func BenchmarkIntoPlain(b *testing.B) {
	msg := sampleUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = msg.IntoPlain(uuidCodec{}, timeCodec{})
	}
}

func BenchmarkIntoPlainErr(b *testing.B) {
	msg := sampleUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.IntoPlainErr(uuidCodecErr{}, timeCodecErr{})
	}
}

func BenchmarkIntoPb(b *testing.B) {
	msg := sampleUserPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = msg.IntoPb(uuidToStringBench{}, timeToTsBench{})
	}
}

func BenchmarkIntoPbErr(b *testing.B) {
	msg := sampleUserPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.IntoPbErr(uuidToStringErrBench{}, timeToTsErrBench{})
	}
}
