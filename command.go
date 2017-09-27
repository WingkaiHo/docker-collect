// command
package main

func calculatePerSecond(duration int64, oldValue uint64, newValue uint64) float64 {
	value := float64(newValue) - float64(oldValue)
	if value < 0 {
		value = 0
	}
	return value / float64(duration)
}
