package analyzer

import "time"

type SizeCalculator struct {
	bytesPerSample int
	retentionDays  int
	scrapeInterval time.Duration
	samplesPerDay  int
}

type SizeConfig struct {
	BytesPerSample int
	RetentionDays  int
	ScrapeInterval time.Duration
}

func NewSizeCalculator(cfg SizeConfig) *SizeCalculator {
	if cfg.BytesPerSample == 0 {
		cfg.BytesPerSample = 2
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 30
	}
	if cfg.ScrapeInterval == 0 {
		cfg.ScrapeInterval = 15 * time.Second
	}

	samplesPerDay := int(24 * time.Hour / cfg.ScrapeInterval)

	return &SizeCalculator{
		bytesPerSample: cfg.BytesPerSample,
		retentionDays:  cfg.RetentionDays,
		scrapeInterval: cfg.ScrapeInterval,
		samplesPerDay:  samplesPerDay,
	}
}

// EstimateSize calculates storage size in bytes for a metric with given cardinality
// Formula: cardinality × samples_per_day × retention_days × bytes_per_sample
func (c *SizeCalculator) EstimateSize(cardinality int) int64 {
	return int64(cardinality) * int64(c.samplesPerDay) * int64(c.retentionDays) * int64(c.bytesPerSample)
}

func (c *SizeCalculator) SamplesPerDay() int {
	return c.samplesPerDay
}

func (c *SizeCalculator) RetentionDays() int {
	return c.retentionDays
}

func (c *SizeCalculator) BytesPerSample() int {
	return c.bytesPerSample
}

func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return formatFloat(float64(bytes)/float64(GB)) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/float64(MB)) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/float64(KB)) + " KB"
	default:
		return formatFloat(float64(bytes)) + " B"
	}
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return string(rune(int(f) + '0'))
	}
	// Simple formatting without imports
	intPart := int64(f)
	fracPart := int64((f - float64(intPart)) * 100)
	if fracPart == 0 {
		return intToStr(intPart)
	}
	return intToStr(intPart) + "." + intToStr(fracPart)
}

func intToStr(n int64) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte(n%10) + '0'}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}
