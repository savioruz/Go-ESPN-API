package shared_test

import (
	"testing"

	"go-espn-api/shared"
)

func BenchmarkConvertStringToBool(b *testing.B) {
	tests := []string{"true", "false", "1", "0", "True", "False", ""}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tt := range tests {
			_ = shared.ConvertStringToBool(tt)
		}
	}
}

func BenchmarkCalculateTotalPage(b *testing.B) {
	tests := []struct {
		total int
		limit int
	}{
		{0, 10},
		{1, 10},
		{100, 10},
		{1000, 10},
		{10000, 10},
		{100, 20},
		{100, 50},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tt := range tests {
			_ = shared.CalculateTotalPage(tt.total, tt.limit)
		}
	}
}

func BenchmarkSingleFilter(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = shared.SingleFilter("123", "id", "users")
	}
}

func BenchmarkBuildCacheKey(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = shared.BuildCacheKey("user", "123", "profile")
	}
}

func BenchmarkBuildCacheKeyWithoutPostfix(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = shared.BuildCacheKey("user")
	}
}

func BenchmarkGenerateUniqueFilename(b *testing.B) {
	filenames := []string{
		"image.jpg",
		"document.pdf",
		"very_long_filename_that_should_be_handled_properly.png",
		"file.txt",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, name := range filenames {
			_ = shared.GenerateUniqueFilename(name)
		}
	}
}
