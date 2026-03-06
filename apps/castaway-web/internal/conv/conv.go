package conv

import "fmt"

// ToInt32 converts an int to int32, returning an error if the value overflows.
func ToInt32(value int) (int32, error) {
	const maxInt32 = int(^uint32(0) >> 1)
	const minInt32 = -maxInt32 - 1
	if value > maxInt32 || value < minInt32 {
		return 0, fmt.Errorf("value %d is out of int32 range", value)
	}
	return int32(value), nil
}
