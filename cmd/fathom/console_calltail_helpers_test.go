package main

// callLogRows is callLogRowSegments concatenated, for tests that count or
// index rows across both generations.
func callLogRows(path string) ([]callLogRow, error) {
	rotated, live, err := callLogRowSegments(path)
	if err != nil {
		return nil, err
	}
	return append(append([]callLogRow(nil), rotated...), live...), nil
}
